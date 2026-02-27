import { BaseAgent } from "./base-agent.js";
import type { AgentContext, AgentOutput } from "./types.js";

export class PMAgent extends BaseAgent {
  protected override buildSystemPrompt(context: AgentContext): string {
    const base = super.buildSystemPrompt(context);
    return [
      base,
      "",
      "## PM-Specific Instructions",
      "You are the Product Manager. Your primary responsibilities:",
      "1. Analyze requirements and produce a clear, structured PRD (Product Requirements Document).",
      "2. Define acceptance criteria for each feature.",
      "3. Break down the project into actionable tasks with priorities.",
      "4. Evaluate test reports against acceptance criteria to determine pass/fail.",
      "5. Output PRDs in markdown format with sections: Overview, Goals, User Stories, Acceptance Criteria, Out of Scope.",
    ].join("\n");
  }

  /** Run the agentic loop with a PRD-focused prompt and return the PRD markdown. */
  async generatePRD(context: AgentContext): Promise<string> {
    const prdContext: AgentContext = {
      ...context,
      conversationHistory: [
        ...context.conversationHistory,
        {
          role: "user",
          content:
            "Based on the requirement, generate a comprehensive PRD in markdown format. " +
            "Include: Overview, Goals, User Stories, Acceptance Criteria, and Out of Scope sections.",
        },
      ],
    };

    const output = await this.runAgenticLoop(prdContext);
    return output.content;
  }

  /** Compare test results against acceptance criteria. */
  async checkAcceptance(
    context: AgentContext,
    testReport: string,
    criteria: string,
  ): Promise<{ passed: boolean; report: string }> {
    const checkContext: AgentContext = {
      ...context,
      conversationHistory: [
        ...context.conversationHistory,
        {
          role: "user",
          content: [
            "Evaluate the following test report against the acceptance criteria.",
            "Respond with a JSON object: { \"passed\": true/false, \"report\": \"...\" }",
            "",
            "## Acceptance Criteria",
            criteria,
            "",
            "## Test Report",
            testReport,
          ].join("\n"),
        },
      ],
    };

    const output = await this.runAgenticLoop(checkContext);

    try {
      const parsed = JSON.parse(output.content);
      return { passed: Boolean(parsed.passed), report: String(parsed.report) };
    } catch {
      // If the LLM didn't return valid JSON, treat as failure with raw content
      return { passed: false, report: output.content };
    }
  }
}
