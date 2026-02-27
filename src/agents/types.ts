import type { LLMMessage, LLMToolCall, ModelTier } from "../llm/types.js";
import type { StageOutput } from "../workflow/types.js";

export type AgentRole = "pm" | "architect" | "coder" | "reviewer" | "tester";

export interface AgentIdentity {
  role: AgentRole;
  name: string;
  modelTier: ModelTier;
  capabilities: string[];
  systemPrompt: string;
}

export interface AgentContext {
  pipelineId: string;
  stageId: string;
  workingDir: string;
  requirement: string;
  /** Artifacts from prior stages (workflow-provided). */
  inputs: StageOutput[];
  /** Rich artifact content for prompt building. */
  artifacts: Array<{ type: string; name: string; content: string }>;
  conversationHistory: LLMMessage[];
  conversationId?: string;
  toolResults?: Array<{ toolCallId: string; result: string }>;
}

export interface AgentOutput {
  content: string;
  toolCalls?: LLMToolCall[];
  /** Files produced by this step (maps to StageOutput). */
  outputs?: StageOutput[];
  artifacts?: Array<{ type: string; name: string; filePath: string }>;
  done: boolean;
}
