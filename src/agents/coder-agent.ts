import { BaseAgent } from "./base-agent.js";
import type { AgentContext, AgentOutput } from "./types.js";

export class CoderAgent extends BaseAgent {
  protected override buildSystemPrompt(context: AgentContext): string {
    const base = super.buildSystemPrompt(context);
    return [
      base,
      "",
      "## Coder-Specific Instructions",
      "You are the Coder. Your primary responsibilities:",
      "1. Implement features based on the PRD and architectural design.",
      "2. Write clean, well-structured code following project conventions.",
      "3. Use filesystem tools to create and modify files.",
      "4. Use shell tools to run builds, linters, and tests.",
      "5. Commit your changes with clear, descriptive commit messages.",
      "6. When done, signal completion with a summary of files changed.",
    ].join("\n");
  }

  /**
   * Run the full implementation loop.
   * The LLM will use filesystem and shell tools to write code,
   * then commit changes when finished.
   */
  async implement(context: AgentContext): Promise<AgentOutput> {
    const implContext: AgentContext = {
      ...context,
      conversationHistory: [
        ...context.conversationHistory,
        {
          role: "user",
          content:
            "Implement the required changes based on the requirement and design artifacts. " +
            "Use the available tools to create/modify files, run tests, and commit your work. " +
            "When finished, provide a summary of all changes made.",
        },
      ],
    };

    return this.runAgenticLoop(implContext);
  }
}
