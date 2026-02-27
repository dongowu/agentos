export interface TokenUsageEntry {
  pipelineId?: string;
  agentRole: string;
  provider: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  estimatedCost: number;
  timestamp: number;
}

export interface TokenSummary {
  totalInputTokens: number;
  totalOutputTokens: number;
  totalEstimatedCost: number;
  byProvider: Record<string, { inputTokens: number; outputTokens: number; estimatedCost: number }>;
  byModel: Record<string, { inputTokens: number; outputTokens: number; estimatedCost: number }>;
}

// Approximate pricing per 1M tokens (input / output)
const PRICING: Record<string, { input: number; output: number }> = {
  'claude-sonnet-4-20250514': { input: 3, output: 15 },
  'claude-haiku-4-20250414': { input: 0.8, output: 4 },
  'gpt-4o': { input: 2.5, output: 10 },
  'gpt-4o-mini': { input: 0.15, output: 0.6 },
  'deepseek-chat': { input: 0.14, output: 0.28 },
  'deepseek-reasoner': { input: 0.55, output: 2.19 },
};

export class TokenTracker {
  private entries: TokenUsageEntry[] = [];

  track(usage: {
    pipelineId?: string;
    agentRole: string;
    provider: string;
    model: string;
    inputTokens: number;
    outputTokens: number;
  }): void {
    const cost = this.estimateCost(usage.model, usage.inputTokens, usage.outputTokens);
    this.entries.push({
      ...usage,
      estimatedCost: cost,
      timestamp: Date.now(),
    });
  }

  getSummary(pipelineId?: string): TokenSummary {
    const filtered = pipelineId
      ? this.entries.filter((e) => e.pipelineId === pipelineId)
      : this.entries;

    const summary: TokenSummary = {
      totalInputTokens: 0,
      totalOutputTokens: 0,
      totalEstimatedCost: 0,
      byProvider: {},
      byModel: {},
    };

    for (const entry of filtered) {
      summary.totalInputTokens += entry.inputTokens;
      summary.totalOutputTokens += entry.outputTokens;
      summary.totalEstimatedCost += entry.estimatedCost;

      if (!summary.byProvider[entry.provider]) {
        summary.byProvider[entry.provider] = { inputTokens: 0, outputTokens: 0, estimatedCost: 0 };
      }
      summary.byProvider[entry.provider].inputTokens += entry.inputTokens;
      summary.byProvider[entry.provider].outputTokens += entry.outputTokens;
      summary.byProvider[entry.provider].estimatedCost += entry.estimatedCost;

      if (!summary.byModel[entry.model]) {
        summary.byModel[entry.model] = { inputTokens: 0, outputTokens: 0, estimatedCost: 0 };
      }
      summary.byModel[entry.model].inputTokens += entry.inputTokens;
      summary.byModel[entry.model].outputTokens += entry.outputTokens;
      summary.byModel[entry.model].estimatedCost += entry.estimatedCost;
    }

    return summary;
  }

  private estimateCost(model: string, inputTokens: number, outputTokens: number): number {
    const pricing = PRICING[model];
    if (!pricing) return 0;
    return (inputTokens * pricing.input + outputTokens * pricing.output) / 1_000_000;
  }
}
