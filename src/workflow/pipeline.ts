import { mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import type { Config, StageDefinition, WorkflowDefinition } from '../config/schema.js';
import type { MessageBus } from '../messaging/message-bus.js';
import type { ConvergenceEngine } from '../messaging/convergence.js';
import type { ToolRegistry } from '../tools/registry.js';
import type { ProjectRepository, PipelineRepository, ArtifactRepository } from '../storage/repository.js';
import { generateId, createLogger } from '../utils/index.js';
import { PipelineError } from '../utils/index.js';
import { StageRunner } from './stage-runner.js';
import type {
  AgentLike,
  AgentContext,
  HumanGate,
  PipelineState,
  PipelineStatus,
  StageOutput,
} from './types.js';

const logger = createLogger('pipeline');

export interface PipelineEngineOptions {
  config: Config;
  agents: Map<string, AgentLike>;
  messageBus: MessageBus;
  convergence: ConvergenceEngine;
  toolRegistry: ToolRegistry;
  projectRepo: ProjectRepository;
  pipelineRepo: PipelineRepository;
  artifactRepo: ArtifactRepository;
  humanGate?: HumanGate;
}

export class PipelineEngine {
  private config: Config;
  private agents: Map<string, AgentLike>;
  private messageBus: MessageBus;
  private convergence: ConvergenceEngine;
  private toolRegistry: ToolRegistry;
  private projectRepo: ProjectRepository;
  private pipelineRepo: PipelineRepository;
  private artifactRepo: ArtifactRepository;
  private humanGate?: HumanGate;
  private stateCache = new Map<string, PipelineState>();

  constructor(opts: PipelineEngineOptions) {
    this.config = opts.config;
    this.agents = opts.agents;
    this.messageBus = opts.messageBus;
    this.convergence = opts.convergence;
    this.toolRegistry = opts.toolRegistry;
    this.projectRepo = opts.projectRepo;
    this.pipelineRepo = opts.pipelineRepo;
    this.artifactRepo = opts.artifactRepo;
    this.humanGate = opts.humanGate;
  }

  async start(requirement: string, workflowId = 'mvp'): Promise<string> {
    const workflow = this.config.workflows[workflowId];
    if (!workflow) {
      throw new PipelineError(`Workflow "${workflowId}" not found`, '', workflowId);
    }

    const now = new Date().toISOString();
    const projectId = generateId('proj');
    const pipelineId = generateId('pipe');
    const workingDir = join(this.config.app.dataDir, 'worktrees', pipelineId);

    this.projectRepo.create({
      id: projectId,
      name: requirement.slice(0, 80),
      requirement,
      status: 'active',
      working_dir: workingDir,
      created_at: now,
      updated_at: now,
    });

    const state: PipelineState = { stages: {} };
    for (const stage of workflow.stages) {
      state.stages[stage.id] = { status: 'pending' };
    }

    this.pipelineRepo.create({
      id: pipelineId,
      project_id: projectId,
      definition_id: workflowId,
      status: 'running',
      current_stage: workflow.stages[0]!.id,
      state_json: JSON.stringify(state),
      created_at: now,
      updated_at: now,
    });

    this.stateCache.set(pipelineId, state);
    await mkdir(workingDir, { recursive: true });

    logger.info({ pipelineId, workflowId }, 'Pipeline started');
    await this.executeLoop(pipelineId, workflow, state, requirement, workingDir);
    return pipelineId;
  }
  private async executeLoop(
    pipelineId: string,
    workflow: WorkflowDefinition,
    state: PipelineState,
    requirement: string,
    workingDir: string,
  ): Promise<void> {
    const runner = new StageRunner(
      this.agents,
      this.messageBus,
      this.convergence,
      this.toolRegistry,
    );

    let stageIndex = this.findCurrentStageIndex(workflow, state);

    while (stageIndex < workflow.stages.length) {
      const stageDef = workflow.stages[stageIndex]!;
      const stageState = state.stages[stageDef.id]!;

      // Skip already completed stages
      if (stageState.status === 'completed') {
        stageIndex++;
        continue;
      }

      stageState.status = 'running';
      stageState.startedAt = new Date().toISOString();
      this.persistState(pipelineId, stageDef.id, state, 'running');

      // Gather inputs from previous stages
      const inputs = this.collectInputs(stageDef, state);

      const context: AgentContext = {
        pipelineId,
        stageId: stageDef.id,
        requirement,
        workingDir,
        inputs,
        artifacts: [],
        conversationHistory: [],
      };

      const result = await runner.run(stageDef, context);

      stageState.completedAt = new Date().toISOString();
      stageState.outputs = result.outputs;
      stageState.error = result.error;

      if (result.status === 'failed') {
        stageState.status = 'failed';
        this.persistState(pipelineId, stageDef.id, state, 'failed');
        logger.error({ pipelineId, stageId: stageDef.id }, 'Stage failed');
        return;
      }

      // Store artifacts
      for (const output of result.outputs) {
        this.artifactRepo.create({
          id: generateId('art'),
          pipeline_id: pipelineId,
          stage_type: stageDef.id,
          artifact_type: output.artifactType,
          name: output.name,
          file_path: output.filePath,
          created_by: stageDef.agent,
          created_at: new Date().toISOString(),
        });
      }
      // Human gate check
      if (stageDef.humanGate && this.humanGate) {
        const gateResult = await this.humanGate.request(pipelineId, stageDef.id, result.outputs);
        if (!gateResult.approved) {
          stageState.status = 'rejected';
          this.persistState(pipelineId, stageDef.id, state, 'paused');

          if (stageDef.onReject?.gotoStage) {
            const targetIdx = workflow.stages.findIndex((s) => s.id === stageDef.onReject!.gotoStage);
            if (targetIdx >= 0) {
              // Reset stages from target onward
              for (let i = targetIdx; i < workflow.stages.length; i++) {
                state.stages[workflow.stages[i]!.id]!.status = 'pending';
              }
              stageIndex = targetIdx;
              logger.info({ pipelineId, gotoStage: stageDef.onReject.gotoStage }, 'Rejected, rewinding');
              continue;
            }
          }
          // No onReject target — pause pipeline
          logger.info({ pipelineId, stageId: stageDef.id }, 'Paused at human gate');
          return;
        }
      }

      stageState.status = 'completed';
      this.persistState(pipelineId, stageDef.id, state, 'running');
      stageIndex++;
    }

    this.persistState(pipelineId, null, state, 'completed');
    logger.info({ pipelineId }, 'Pipeline completed');
  }
  async resume(pipelineId: string): Promise<void> {
    const record = this.pipelineRepo.findById(pipelineId);
    if (!record) {
      throw new PipelineError(`Pipeline "${pipelineId}" not found`, pipelineId, '');
    }

    const workflow = this.config.workflows[record.definition_id];
    if (!workflow) {
      throw new PipelineError(`Workflow "${record.definition_id}" not found`, pipelineId, '');
    }

    const state: PipelineState = record.state_json
      ? JSON.parse(record.state_json)
      : { stages: {} };
    this.stateCache.set(pipelineId, state);

    const project = this.projectRepo.findById(record.project_id);
    const workingDir = project?.working_dir ?? join(this.config.app.dataDir, 'worktrees', pipelineId);
    const requirement = project?.requirement ?? '';

    logger.info({ pipelineId }, 'Resuming pipeline');
    await this.executeLoop(pipelineId, workflow, state, requirement, workingDir);
  }

  getStatus(pipelineId: string): { status: PipelineStatus; state: PipelineState; currentStage?: string } | null {
    const record = this.pipelineRepo.findById(pipelineId);
    if (!record) return null;

    const state: PipelineState = record.state_json
      ? JSON.parse(record.state_json)
      : { stages: {} };

    return {
      status: record.status as PipelineStatus,
      state,
      currentStage: record.current_stage ?? undefined,
    };
  }

  private findCurrentStageIndex(workflow: WorkflowDefinition, state: PipelineState): number {
    for (let i = 0; i < workflow.stages.length; i++) {
      const s = state.stages[workflow.stages[i]!.id];
      if (!s || s.status !== 'completed') return i;
    }
    return workflow.stages.length;
  }

  private collectInputs(stage: StageDefinition, state: PipelineState): StageOutput[] {
    if (!stage.inputs) return [];
    const inputs: StageOutput[] = [];
    for (const inp of stage.inputs) {
      const prev = state.stages[inp.fromStage];
      if (prev?.outputs) {
        inputs.push(...prev.outputs.filter((o) => o.artifactType === inp.type));
      }
    }
    return inputs;
  }

  private persistState(
    pipelineId: string,
    currentStage: string | null,
    state: PipelineState,
    status: PipelineStatus,
  ): void {
    this.stateCache.set(pipelineId, state);
    this.pipelineRepo.updateState(pipelineId, JSON.stringify(state));
    this.pipelineRepo.updateStatus(pipelineId, status);
    if (currentStage) {
      this.pipelineRepo.updateStage(pipelineId, currentStage);
    }
  }
}
