import type { LLMMessage, LLMResponse, LLMToolCall, LLMToolDefinition, ModelTier } from "../llm/types.js";
import type { Message } from "../messaging/types.js";
import type { Tool, ToolDefinition, ToolResult } from "../tools/types.js";
import type { AgentContext, AgentIdentity, AgentOutput } from "./types.js";

/** Duck-typed LLMRouter to avoid tight coupling. */
export interface LLMRouterLike {
  route(tier: ModelTier, messages: LLMMessage[], options?: { tools?: LLMToolDefinition[]; maxTokens?: number }): Promise<LLMResponse>;
}

/** Duck-typed MessageBus. */
export interface MessageBusLike {
  send(msg: Omit<Message, "id" | "timestamp">): Promise<Message>;
}

/** Duck-typed ToolRegistry. */
export interface ToolRegistryLike {
  getTools(capabilities: string[]): Tool[];
  getDefinitions(capabilities: string[]): ToolDefinition[];
  getTool(name: string): Tool | undefined;
}

export abstract class BaseAgent {
  readonly identity: AgentIdentity;
  protected llmRouter: LLMRouterLike;
  protected messageBus: MessageBusLike;
  protected toolRegistry: ToolRegistryLike;

  /** Expose role at top level so BaseAgent satisfies workflow AgentLike. */
  get role(): string {
    return this.identity.role;
  }

  constructor(
    identity: AgentIdentity,
    llmRouter: LLMRouterLike,
    messageBus: MessageBusLike,
    toolRegistry: ToolRegistryLike,
  ) {
    this.identity = identity;
    this.llmRouter = llmRouter;
    this.messageBus = messageBus;
    this.toolRegistry = toolRegistry;
  }

  /** Single LLM turn: build prompt, call router, parse response. */
  async step(context: AgentContext): Promise<AgentOutput> {
    const systemPrompt = this.buildSystemPrompt(context);
    const messages = this.buildMessages(systemPrompt, context);
    const toolDefs = this.toolRegistry.getDefinitions(this.identity.capabilities);

    const response = await this.llmRouter.route(
      this.identity.modelTier,
      messages,
      toolDefs.length > 0 ? { tools: toolDefs as LLMToolDefinition[] } : undefined,
    );

    if (response.toolCalls && response.toolCalls.length > 0) {
      return {
        content: response.content,
        toolCalls: response.toolCalls,
        done: false,
      };
    }

    return { content: response.content, done: true };
  }

  /** Execute an array of tool calls and collect results. */
  async executeToolCalls(
    toolCalls: LLMToolCall[],
  ): Promise<Array<{ toolCallId: string; result: string }>> {
    const results: Array<{ toolCallId: string; result: string }> = [];

    for (const call of toolCalls) {
      const tool = this.toolRegistry.getTool(call.name);
      if (!tool) {
        results.push({ toolCallId: call.id, result: `Error: unknown tool "${call.name}"` });
        continue;
      }
      let toolResult: ToolResult;
      try {
        toolResult = await tool.execute(call.arguments);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        results.push({ toolCallId: call.id, result: `Error: ${msg}` });
        continue;
      }
      results.push({
        toolCallId: call.id,
        result: toolResult.success ? toolResult.output : `Error: ${toolResult.error ?? toolResult.output}`,
      });
    }

    return results;
  }
  /**
   * Run a full agentic loop: step → tool calls → feed results → repeat.
   * Stops when the agent signals done or maxIterations is reached.
   */
  async runAgenticLoop(context: AgentContext, maxIterations = 10): Promise<AgentOutput> {
    const ctx = { ...context, conversationHistory: [...context.conversationHistory] };

    for (let i = 0; i < maxIterations; i++) {
      const output = await this.step(ctx);

      if (output.done) {
        return output;
      }

      if (output.toolCalls && output.toolCalls.length > 0) {
        const results = await this.executeToolCalls(output.toolCalls);
        ctx.toolResults = results;

        // Append assistant message + tool results to conversation history
        ctx.conversationHistory.push({ role: "assistant", content: output.content || "" });
        const toolSummary = results
          .map((r) => `[tool:${r.toolCallId}] ${r.result}`)
          .join("\n");
        ctx.conversationHistory.push({ role: "user", content: toolSummary });
      }
    }

    // Safety: return last content if we hit the iteration limit
    return { content: "Max iterations reached.", done: true };
  }

  /**
   * Handle an incoming message. Accepts unknown to satisfy workflow AgentLike.
   * If msg is a Message, uses its body; otherwise stringifies it.
   */
  async handleMessage(msg: unknown, context: AgentContext): Promise<unknown> {
    const message = msg as Partial<Message> | undefined;
    const body = message?.body ?? (typeof msg === "string" ? msg : JSON.stringify(msg));
    context.conversationHistory.push({ role: "user", content: body });

    const output = await this.step(context);
    if (!output.content) return null;

    // If the msg looks like a Message with conversationId, send a reply
    if (message?.conversationId) {
      return this.messageBus.send({
        conversationId: message.conversationId,
        parentId: message.id,
        type: "discuss",
        routing: "unicast",
        fromRole: this.identity.role,
        toRole: message.fromRole,
        subject: message.subject,
        body: output.content,
      });
    }

    return output.content;
  }

  /** Build the full system prompt from identity + context. */
  protected buildSystemPrompt(context: AgentContext): string {
    const parts = [this.identity.systemPrompt];

    parts.push(`\n## Current Context`);
    parts.push(`- Stage: ${context.stageId}`);
    parts.push(`- Working directory: ${context.workingDir}`);
    parts.push(`\n## Requirement\n${context.requirement}`);

    if (context.artifacts && context.artifacts.length > 0) {
      parts.push(`\n## Available Artifacts`);
      for (const a of context.artifacts) {
        parts.push(`### ${a.type}: ${a.name}\n${a.content}`);
      }
    }

    return parts.join("\n");
  }

  /** Assemble the messages array for the LLM call. */
  private buildMessages(systemPrompt: string, context: AgentContext): LLMMessage[] {
    const msgs: LLMMessage[] = [{ role: "system", content: systemPrompt }];

    for (const h of context.conversationHistory) {
      msgs.push(h);
    }

    // Append tool results as a user message if present
    if (context.toolResults && context.toolResults.length > 0) {
      const summary = context.toolResults
        .map((r) => `[tool:${r.toolCallId}] ${r.result}`)
        .join("\n");
      msgs.push({ role: "user", content: summary });
    }

    return msgs;
  }
}