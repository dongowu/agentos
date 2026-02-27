import Anthropic from '@anthropic-ai/sdk';
import type {
  LLMMessage,
  LLMOptions,
  LLMProvider,
  LLMResponse,
  LLMToolCall,
} from '../types.js';

export class AnthropicProvider implements LLMProvider {
  readonly name = 'anthropic';
  private client: Anthropic;
  private model: string;

  constructor(apiKey: string, model: string, baseURL?: string) {
    this.client = new Anthropic({ apiKey, baseURL });
    this.model = model;
  }

  async chat(messages: LLMMessage[], options?: LLMOptions): Promise<LLMResponse> {
    const systemMsg = messages.find((m) => m.role === 'system');
    const nonSystem = messages.filter((m) => m.role !== 'system') as Array<{
      role: 'user' | 'assistant';
      content: string;
    }>;

    const params: Anthropic.MessageCreateParams = {
      model: this.model,
      max_tokens: options?.maxTokens ?? 4096,
      messages: nonSystem,
    };

    if (systemMsg) {
      params.system = systemMsg.content;
    }
    if (options?.temperature !== undefined) {
      params.temperature = options.temperature;
    }
    if (options?.tools?.length) {
      params.tools = options.tools.map((t) => ({
        name: t.name,
        description: t.description,
        input_schema: t.parameters as Anthropic.Tool['input_schema'],
      }));
    }

    const response = await this.client.messages.create(params);

    let content = '';
    const toolCalls: LLMToolCall[] = [];

    for (const block of response.content) {
      if (block.type === 'text') {
        content += block.text;
      } else if (block.type === 'tool_use') {
        toolCalls.push({
          id: block.id,
          name: block.name,
          arguments: block.input as Record<string, unknown>,
        });
      }
    }

    return {
      content,
      toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
      inputTokens: response.usage.input_tokens,
      outputTokens: response.usage.output_tokens,
      model: this.model,
      provider: this.name,
    };
  }

  async *stream(messages: LLMMessage[], options?: LLMOptions): AsyncIterable<string> {
    const systemMsg = messages.find((m) => m.role === 'system');
    const nonSystem = messages.filter((m) => m.role !== 'system') as Array<{
      role: 'user' | 'assistant';
      content: string;
    }>;

    const params: Anthropic.MessageCreateParams = {
      model: this.model,
      max_tokens: options?.maxTokens ?? 4096,
      messages: nonSystem,
      stream: true,
    };

    if (systemMsg) {
      params.system = systemMsg.content;
    }
    if (options?.temperature !== undefined) {
      params.temperature = options.temperature;
    }

    const stream = this.client.messages.stream({
      ...params,
      stream: undefined,
    });

    for await (const event of stream) {
      if (
        event.type === 'content_block_delta' &&
        event.delta.type === 'text_delta'
      ) {
        yield event.delta.text;
      }
    }
  }
}