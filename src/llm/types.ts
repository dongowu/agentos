export interface LLMMessage {
  role: 'system' | 'user' | 'assistant';
  content: string;
}

export interface LLMToolDefinition {
  name: string;
  description: string;
  parameters: Record<string, unknown>; // JSON Schema
}

export interface LLMToolCall {
  id: string;
  name: string;
  arguments: Record<string, unknown>;
}

export interface LLMResponse {
  content: string;
  toolCalls?: LLMToolCall[];
  inputTokens: number;
  outputTokens: number;
  model: string;
  provider: string;
}

export interface LLMOptions {
  temperature?: number;
  maxTokens?: number;
  tools?: LLMToolDefinition[];
}

export type ModelTier = 'reasoning' | 'balanced' | 'fast';

export interface LLMProvider {
  name: string;
  chat(messages: LLMMessage[], options?: LLMOptions): Promise<LLMResponse>;
  stream(messages: LLMMessage[], options?: LLMOptions): AsyncIterable<string>;
}
