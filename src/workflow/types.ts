export type PipelineStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed';
export type StageStatus = 'pending' | 'running' | 'completed' | 'failed' | 'rejected';

export interface StageResult {
  status: StageStatus;
  outputs: StageOutput[];
  error?: string;
}

export interface StageOutput {
  artifactType: string;
  name: string;
  filePath: string;
}

export interface PipelineState {
  stages: Record<string, {
    status: StageStatus;
    startedAt?: string;
    completedAt?: string;
    outputs?: StageOutput[];
    error?: string;
  }>;
}

/** Generic agent interface to avoid circular dependency with agents module. */
export interface AgentLike {
  role: string;
  step(context: AgentContext): Promise<AgentStepResult>;
  handleMessage(msg: unknown, context: AgentContext): Promise<unknown>;
}

export interface AgentContext {
  pipelineId: string;
  stageId: string;
  requirement: string;
  workingDir: string;
  inputs: StageOutput[];
  /** Rich artifact content for prompt building. */
  artifacts: Array<{ type: string; name: string; content: string }>;
  conversationHistory: Array<{ role: string; content: string }>;
  conversationId?: string;
}

export interface AgentStepResult {
  content: string;
  toolCalls?: { id?: string; name: string; arguments: Record<string, unknown> }[];
  outputs?: StageOutput[];
  done: boolean;
}

export interface HumanGate {
  request(pipelineId: string, stageId: string, outputs: StageOutput[]): Promise<HumanGateResult>;
}

export interface HumanGateResult {
  approved: boolean;
  feedback?: string;
}
