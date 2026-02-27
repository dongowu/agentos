import { resolve } from 'node:path';
import { loadConfig } from './config/index.js';
import type { Config } from './config/schema.js';
import { initDatabase } from './storage/database.js';
import {
  ProjectRepository,
  PipelineRepository,
  MessageRepository,
  ConversationRepository,
  ArtifactRepository,
  TokenUsageRepository,
} from './storage/repository.js';
import { AnthropicProvider } from './llm/providers/anthropic.js';
import { OpenAIProvider } from './llm/providers/openai.js';
import { OpenAIResponsesProvider } from './llm/providers/openai-responses.js';
import { LLMRouter, type RouterConfig } from './llm/router.js';
import { TokenTracker } from './llm/token-tracker.js';
import type { LLMProvider } from './llm/types.js';
import { MessageBus, type MessageRepository as MessageRepoInterface } from './messaging/message-bus.js';
import { ConvergenceEngine } from './messaging/convergence.js';
import type { Message, Conversation, ConversationStatus } from './messaging/types.js';
import { ToolRegistry } from './tools/registry.js';
import { FilesystemTool } from './tools/filesystem.js';
import { GitTool } from './tools/git.js';
import { ShellTool } from './tools/shell.js';
import { HumanGate } from './human/gate.js';
import { PMAgent } from './agents/pm-agent.js';
import { CoderAgent } from './agents/coder-agent.js';
import type { AgentIdentity } from './agents/types.js';
import { PipelineEngine } from './workflow/pipeline.js';
import { createLogger } from './utils/logger.js';

const logger = createLogger('bootstrap');

export interface OrchestratorSystem {
  config: Config;
  repositories: {
    projects: ProjectRepository;
    pipelines: PipelineRepository;
    messages: MessageRepository;
    conversations: ConversationRepository;
    artifacts: ArtifactRepository;
    tokenUsage: TokenUsageRepository;
  };
  llmRouter: LLMRouter;
  tokenTracker: TokenTracker;
  messageBus: MessageBus;
  convergence: ConvergenceEngine;
  toolRegistry: ToolRegistry;
  humanGate: HumanGate;
  agents: Map<string, PMAgent | CoderAgent>;
  pipelineEngine: PipelineEngine;
}

export async function bootstrap(configDir?: string): Promise<OrchestratorSystem> {
  // 1. Load config
  const config = loadConfig(configDir);
  logger.info('Config loaded');

  // 2. Init database
  const dbPath = resolve(config.app.dataDir, 'orchestrator.db');
  const db = initDatabase(dbPath);
  logger.info('Database initialized');

  // 3. Create repositories
  const repos = {
    projects: new ProjectRepository(db),
    pipelines: new PipelineRepository(db),
    messages: new MessageRepository(db),
    conversations: new ConversationRepository(db),
    artifacts: new ArtifactRepository(db),
    tokenUsage: new TokenUsageRepository(db),
  };

  // 4. Create LLM providers
  const providers = new Map<string, LLMProvider>();
  const env = config.env ?? {};

  if (env.anthropicApiKey) {
    // Create one provider per model used in tiers
    for (const [, tier] of Object.entries(config.llm.tiers)) {
      for (const p of tier.providers) {
        if (p.name === 'anthropic') {
          const key = `anthropic:${p.model}`;
          if (!providers.has(key)) {
            providers.set(key, new AnthropicProvider(env.anthropicApiKey, p.model, env.anthropicBaseUrl));
          }
        }
      }
    }
  }

  if (env.openaiApiKey) {
    const useResponses = env.openaiWireApi === 'responses';
    for (const [, tier] of Object.entries(config.llm.tiers)) {
      for (const p of tier.providers) {
        if (p.name === 'openai') {
          const key = `openai:${p.model}`;
          if (!providers.has(key)) {
            if (useResponses && env.openaiBaseUrl) {
              providers.set(key, new OpenAIResponsesProvider(env.openaiApiKey, p.model, env.openaiBaseUrl));
            } else {
              providers.set(key, new OpenAIProvider(env.openaiApiKey, p.model, env.openaiBaseUrl));
            }
          }
        }
      }
    }
  }

  if (env.deepseekApiKey) {
    const baseUrl = env.deepseekBaseUrl ?? 'https://api.deepseek.com';
    for (const [, tier] of Object.entries(config.llm.tiers)) {
      for (const p of tier.providers) {
        if (p.name === 'deepseek') {
          const key = `deepseek:${p.model}`;
          if (!providers.has(key)) {
            providers.set(key, new OpenAIProvider(env.deepseekApiKey, p.model, baseUrl));
          }
        }
      }
    }
  }

  // 5. Build router config — map tier providers to "provider:model" keys
  const routerConfig: RouterConfig = {
    tiers: {
      reasoning: { providers: config.llm.tiers.reasoning.providers.map((p) => `${p.name}:${p.model}`) },
      balanced: { providers: config.llm.tiers.balanced.providers.map((p) => `${p.name}:${p.model}`) },
      fast: { providers: config.llm.tiers.fast.providers.map((p) => `${p.name}:${p.model}`) },
    },
    cooldownMs: config.llm.cooldownMs,
  };
  const llmRouter = new LLMRouter(routerConfig, providers);
  const tokenTracker = new TokenTracker();
  logger.info(`LLM router ready with ${providers.size} provider(s)`);

  // 6. Create message bus with repository adapter
  const messageRepoAdapter: MessageRepoInterface = {
    async saveMessage(msg: Message) {
      repos.messages.create({
        id: msg.id,
        conversation_id: msg.conversationId,
        parent_id: msg.parentId ?? null,
        type: msg.type,
        routing: msg.routing,
        from_role: msg.fromRole,
        to_role: msg.toRole ?? null,
        subject: msg.subject ?? null,
        body: msg.body,
        metadata_json: msg.metadata ? JSON.stringify(msg.metadata) : null,
        timestamp: msg.timestamp,
      });
    },
    async getMessages(conversationId: string) {
      const rows = repos.messages.findByConversationId(conversationId);
      return rows.map((r) => ({
        id: r.id,
        conversationId: r.conversation_id,
        parentId: r.parent_id ?? undefined,
        type: r.type as Message['type'],
        routing: r.routing as Message['routing'],
        fromRole: r.from_role as Message['fromRole'],
        toRole: r.to_role as Message['toRole'],
        subject: r.subject ?? undefined,
        body: r.body,
        metadata: r.metadata_json ? JSON.parse(r.metadata_json) : undefined,
        timestamp: r.timestamp,
      }));
    },
    async saveConversation(conv: Conversation) {
      repos.conversations.create({
        id: conv.id,
        pipeline_id: conv.pipelineId,
        stage_id: conv.stageId ?? null,
        topic: conv.topic,
        participants_json: JSON.stringify(conv.participants),
        status: conv.status,
        round_count: conv.roundCount,
        max_rounds: conv.maxRounds,
        created_at: new Date().toISOString(),
      });
    },
    async getConversation(id: string) {
      const r = repos.conversations.findById(id);
      if (!r) return null;
      return {
        id: r.id,
        pipelineId: r.pipeline_id,
        stageId: r.stage_id ?? undefined,
        topic: r.topic,
        participants: JSON.parse(r.participants_json),
        status: r.status as Conversation['status'],
        roundCount: r.round_count,
        maxRounds: r.max_rounds,
      };
    },
    async updateConversationStatus(id: string, status: ConversationStatus, roundCount?: number) {
      repos.conversations.updateStatus(id, status);
      if (roundCount !== undefined) {
        repos.conversations.incrementRound(id);
      }
    },
  };

  const messageBus = new MessageBus(messageRepoAdapter);
  const convergence = new ConvergenceEngine(messageBus);
  logger.info('Message bus ready');

  // 7. Create tool registry
  const workingDir = resolve(config.app.dataDir, 'worktrees');
  const toolRegistry = new ToolRegistry();
  toolRegistry.register('filesystem', new FilesystemTool(workingDir));
  toolRegistry.register('git', new GitTool(workingDir));
  toolRegistry.register('shell', new ShellTool(workingDir));
  logger.info('Tool registry ready');

  // 8. Human gate
  const humanGate = new HumanGate();

  // 9. Create agents (MVP: PM + Coder)
  const agents = new Map<string, PMAgent | CoderAgent>();
  const agentConfigs = config.agents;

  if (agentConfigs.pm) {
    const identity: AgentIdentity = {
      role: 'pm',
      name: agentConfigs.pm.name,
      modelTier: agentConfigs.pm.modelTier as AgentIdentity['modelTier'],
      capabilities: agentConfigs.pm.capabilities,
      systemPrompt: agentConfigs.pm.systemPrompt,
    };
    agents.set('pm', new PMAgent(identity, llmRouter, messageBus, toolRegistry));
  }

  if (agentConfigs.coder) {
    const identity: AgentIdentity = {
      role: 'coder',
      name: agentConfigs.coder.name,
      modelTier: agentConfigs.coder.modelTier as AgentIdentity['modelTier'],
      capabilities: agentConfigs.coder.capabilities,
      systemPrompt: agentConfigs.coder.systemPrompt,
    };
    agents.set('coder', new CoderAgent(identity, llmRouter, messageBus, toolRegistry));
  }

  logger.info(`Agents ready: ${[...agents.keys()].join(', ')}`);

  // 10. Pipeline engine
  const pipelineEngine = new PipelineEngine({
    config,
    agents,
    messageBus,
    convergence,
    toolRegistry,
    projectRepo: repos.projects,
    pipelineRepo: repos.pipelines,
    artifactRepo: repos.artifacts,
    humanGate,
  });
  logger.info('Pipeline engine ready');

  return {
    config,
    repositories: repos,
    llmRouter,
    tokenTracker,
    messageBus,
    convergence,
    toolRegistry,
    humanGate,
    agents,
    pipelineEngine,
  };
}
