import OpenAI from 'openai';
import type {
  LLMMessage,
  LLMOptions,
  LLMProvider,
  LLMResponse,
  LLMToolCall,
} from '../types.js';

/**
 * Provider for OpenAI Responses API (POST /responses).
 * Used by proxies like xychatai that expose the Responses wire format.
 */
export class OpenAIResponsesProvider implements LLMProvider {
  readonly name: string;
  private client: OpenAI;
  private model: string;

  constructor(apiKey: string, model: string, baseURL: string, name = 'openai-responses') {
    this.client = new OpenAI({ apiKey, baseURL });
    this.model = model;
    this.name = name;
  }

  async chat(messages: LLMMessage[], options?: LLMOptions): Promise<LLMResponse> {
    // Convert LLMMessage[] to Responses API input format
    const input: OpenAI.Responses.ResponseInput = [];

    for (const msg of messages) {
      if (msg.role === 'system') {
        input.push({ role: 'developer', content: msg.content });
      } else {
        input.push({ role: msg.role, content: msg.content });
      }
    }

    const params: OpenAI.Responses.ResponseCreateParamsNonStreaming = {
      model: this.model,
      input,
      stream: false,
    };

    if (options?.temperature !== undefined) {
      params.temperature = options.temperature;
    }
    if (options?.maxTokens !== undefined) {
      params.max_output_tokens = options.maxTokens;
    }
    if (options?.tools?.length) {
      params.tools = options.tools.map((t) => ({
        type: 'function' as const,
        name: t.name,
        description: t.description,
        parameters: t.parameters,
        strict: false,
      }));
    }

    const response = await this.client.responses.create(params);

    let content = '';
    const toolCalls: LLMToolCall[] = [];

    for (const item of response.output) {
      if (item.type === 'message') {
        for (const part of item.content) {
          if (part.type === 'output_text') {
            content += part.text;
          }
        }
      } else if (item.type === 'function_call') {
        toolCalls.push({
          id: item.call_id,
          name: item.name,
          arguments: JSON.parse(item.arguments) as Record<string, unknown>,
        });
      }
    }

    return {
      content,
      toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
      inputTokens: response.usage?.input_tokens ?? 0,
      outputTokens: response.usage?.output_tokens ?? 0,
      model: this.model,
      provider: this.name,
    };
  }

  async *stream(messages: LLMMessage[], options?: LLMOptions): AsyncIterable<string> {
    const input: OpenAI.Responses.ResponseInput = [];

    for (const msg of messages) {
      if (msg.role === 'system') {
        input.push({ role: 'developer', content: msg.content });
      } else {
        input.push({ role: msg.role, content: msg.content });
      }
    }

    const params: OpenAI.Responses.ResponseCreateParamsStreaming = {
      model: this.model,
      input,
      stream: true,
    };

    if (options?.temperature !== undefined) {
      params.temperature = options.temperature;
    }
    if (options?.maxTokens !== undefined) {
      params.max_output_tokens = options.maxTokens;
    }

    const stream = await this.client.responses.create(params);

    for await (const event of stream) {
      if (event.type === 'response.output_text.delta') {
        yield event.delta;
      }
    }
  }
}
