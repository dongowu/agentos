import type { StageDefinition } from '../config/schema.js';
import type { MessageBus } from '../messaging/message-bus.js';
import type { ConvergenceEngine } from '../messaging/convergence.js';
import type { ToolRegistry } from '../tools/registry.js';
import type { AgentRole } from '../messaging/types.js';
import type {
  AgentLike,
  AgentContext,
  StageResult,
  StageOutput,
} from './types.js';
import { createLogger } from '../utils/index.js';
import { PipelineError } from '../utils/index.js';

const logger = createLogger('stage-runner');
const MAX_AGENTIC_ITERATIONS = 10;

export class StageRunner {
  constructor(
    private agents: Map<string, AgentLike>,
    private messageBus: MessageBus,
    private convergence: ConvergenceEngine,
    private toolRegistry: ToolRegistry,
  ) {}

  async run(stage: StageDefinition, context: AgentContext): Promise<StageResult> {
    const agent = this.agents.get(stage.agent);
    if (!agent) {
      throw new PipelineError(
        `Agent "${stage.agent}" not found for stage "${stage.id}"`,
        context.pipelineId,
        stage.id,
      );
    }

    logger.info({ stageId: stage.id, agent: stage.agent }, 'Starting stage');

    // --- Discussion phase (if collaborators exist) ---
    if (stage.collaborators && stage.collaborators.length > 0) {
      await this.runDiscussion(stage, context);
    }

    // --- Agentic loop ---
    return this.runAgenticLoop(agent, stage, context);
  }
  private async runDiscussion(
    stage: StageDefinition,
    context: AgentContext,
  ): Promise<void> {
    const participants = [stage.agent, ...(stage.collaborators ?? [])] as AgentRole[];

    const conv = await this.messageBus.createConversation({
      pipelineId: context.pipelineId,
      stageId: stage.id,
      topic: `Stage: ${stage.name}`,
      participants,
      maxRounds: 5,
    });

    context.conversationId = conv.id;

    // Lead agent kicks off the discussion
    const leadAgent = this.agents.get(stage.agent);
    if (leadAgent) {
      const initResult = await leadAgent.handleMessage(
        { type: 'discuss', topic: stage.name, inputs: context.inputs },
        context,
      );
      await this.messageBus.send({
        conversationId: conv.id,
        type: 'discuss',
        routing: 'meeting',
        fromRole: stage.agent as AgentRole,
        body: String(initResult),
      });
    }

    // Discussion rounds
    for (let round = 0; round < conv.maxRounds; round++) {
      for (const role of stage.collaborators ?? []) {
        const collaborator = this.agents.get(role);
        if (!collaborator) continue;

        const messages = await this.messageBus.getMessages(conv.id);
        const response = await collaborator.handleMessage(
          { type: 'discuss', history: messages },
          context,
        );
        await this.messageBus.send({
          conversationId: conv.id,
          type: 'propose',
          routing: 'meeting',
          fromRole: role as AgentRole,
          body: String(response),
        });
      }

      const result = await this.convergence.checkConvergence(conv.id);
      if (result.converged) {
        logger.info({ stageId: stage.id, round }, 'Discussion converged');
        return;
      }
      if (result.expired || result.shouldEscalate) {
        logger.warn({ stageId: stage.id }, 'Discussion expired, proceeding with latest state');
        return;
      }
    }
  }
  private async runAgenticLoop(
    agent: AgentLike,
    stage: StageDefinition,
    context: AgentContext,
  ): Promise<StageResult> {
    const outputs: StageOutput[] = [];

    for (let i = 0; i < MAX_AGENTIC_ITERATIONS; i++) {
      let stepResult;
      try {
        stepResult = await agent.step(context);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        logger.error({ stageId: stage.id, iteration: i, err }, 'Agent step failed');
        return { status: 'failed', outputs, error: message };
      }

      // Execute any tool calls
      if (stepResult.toolCalls && stepResult.toolCalls.length > 0) {
        for (const call of stepResult.toolCalls) {
          const tool = this.toolRegistry.getTool(call.name);
          if (!tool) {
            logger.warn({ tool: call.name }, 'Tool not found, skipping');
            continue;
          }
          const result = await tool.execute(call.arguments);
          logger.debug({ tool: call.name, success: result.success }, 'Tool executed');
        }
      }

      // Collect outputs
      if (stepResult.outputs) {
        outputs.push(...stepResult.outputs);
      }

      if (stepResult.done) {
        logger.info({ stageId: stage.id, iterations: i + 1 }, 'Stage completed');
        return { status: 'completed', outputs };
      }
    }

    logger.warn({ stageId: stage.id }, 'Max iterations reached');
    return { status: 'completed', outputs };
  }
}
