import type {
  LLMMessage,
  LLMOptions,
  LLMProvider,
  LLMResponse,
  ModelTier,
} from './types.js';

interface ProviderHealth {
  healthy: boolean;
  lastFailure: number;
}

export interface TierConfig {
  providers: string[]; // ordered provider keys
}

export interface RouterConfig {
  tiers: Record<ModelTier, TierConfig>;
  cooldownMs?: number;
}

export class LLMRouter {
  private providers: Map<string, LLMProvider>;
  private config: RouterConfig;
  private health: Map<string, ProviderHealth> = new Map();
  private cooldownMs: number;

  constructor(config: RouterConfig, providers: Map<string, LLMProvider>) {
    this.config = config;
    this.providers = providers;
    this.cooldownMs = config.cooldownMs ?? 30_000;
  }

  async route(
    tier: ModelTier,
    messages: LLMMessage[],
    options?: LLMOptions,
  ): Promise<LLMResponse> {
    const tierConfig = this.config.tiers[tier];
    if (!tierConfig) {
      throw new Error(`No configuration for tier: ${tier}`);
    }

    const errors: Error[] = [];

    for (const providerKey of tierConfig.providers) {
      const provider = this.providers.get(providerKey);
      if (!provider) continue;

      if (!this.isHealthy(providerKey)) continue;

      try {
        const response = await provider.chat(messages, options);
        return response;
      } catch (err) {
        const error = err instanceof Error ? err : new Error(String(err));
        this.markUnhealthy(providerKey);
        errors.push(error);
      }
    }

    throw new AggregateError(
      errors,
      `All providers failed for tier "${tier}": ${errors.map((e) => e.message).join('; ')}`,
    );
  }

  private isHealthy(key: string): boolean {
    const status = this.health.get(key);
    if (!status || status.healthy) return true;
    // Check if cooldown has elapsed
    if (Date.now() - status.lastFailure >= this.cooldownMs) {
      status.healthy = true;
      return true;
    }
    return false;
  }

  private markUnhealthy(key: string): void {
    this.health.set(key, { healthy: false, lastFailure: Date.now() });
  }

  resetHealth(providerKey?: string): void {
    if (providerKey) {
      this.health.delete(providerKey);
    } else {
      this.health.clear();
    }
  }
}
