import { z } from "zod";

// --- App ---

export const AppConfigSchema = z.object({
  name: z.string(),
  dataDir: z.string(),
  logLevel: z.enum(["debug", "info", "warn", "error"]).default("info"),
});

// --- LLM ---

export const LLMProviderSchema = z.object({
  name: z.enum(["anthropic", "openai", "deepseek"]),
  model: z.string(),
});

export const LLMTierSchema = z.object({
  providers: z.array(LLMProviderSchema).min(1),
});

export const LLMConfigSchema = z.object({
  defaultTimeout: z.number().positive(),
  maxRetries: z.number().int().nonnegative(),
  cooldownMs: z.number().nonnegative(),
  tiers: z.object({
    reasoning: LLMTierSchema,
    balanced: LLMTierSchema,
    fast: LLMTierSchema,
  }),
});

// --- Agents ---

export const AgentDefinitionSchema = z.object({
  role: z.string(),
  name: z.string(),
  modelTier: z.enum(["reasoning", "balanced", "fast"]),
  capabilities: z.array(z.string()),
  systemPrompt: z.string(),
});

// --- Workflow stages ---

export const StageInputSchema = z.object({
  fromStage: z.string(),
  type: z.string(),
});

export const StageOutputSchema = z.object({
  type: z.string(),
  name: z.string(),
});

export const StageOnRejectSchema = z.object({
  gotoStage: z.string(),
});

export const StageDefinitionSchema = z.object({
  id: z.string(),
  name: z.string(),
  agent: z.string(),
  collaborators: z.array(z.string()).optional(),
  humanGate: z.boolean().optional(),
  inputs: z.array(StageInputSchema).optional(),
  outputs: z.array(StageOutputSchema).optional(),
  onReject: StageOnRejectSchema.optional(),
});

// --- Workflow ---

export const WorkflowDefinitionSchema = z.object({
  name: z.string(),
  stages: z.array(StageDefinitionSchema).min(1),
});

export const WorkflowConfigSchema = z.object({
  maxDiscussionRounds: z.number().int().positive().optional(),
  discussionTimeoutMs: z.number().positive().optional(),
  humanGates: z.array(z.string()).optional(),
});

// --- Web ---

export const WebConfigSchema = z.object({
  port: z.number().int().positive(),
  host: z.string(),
});

// --- Env ---

export const EnvConfigSchema = z.object({
  anthropicApiKey: z.string().optional(),
  anthropicBaseUrl: z.string().optional(),
  openaiApiKey: z.string().optional(),
  openaiBaseUrl: z.string().optional(),
  openaiWireApi: z.enum(['chat', 'responses']).optional(),
  deepseekApiKey: z.string().optional(),
  deepseekBaseUrl: z.string().optional(),
});

// --- Full config ---

export const FullConfigSchema = z.object({
  app: AppConfigSchema,
  llm: LLMConfigSchema,
  agents: z.record(z.string(), AgentDefinitionSchema),
  workflows: z.record(z.string(), WorkflowDefinitionSchema),
  workflow: WorkflowConfigSchema.optional(),
  web: WebConfigSchema,
  env: EnvConfigSchema.optional(),
});

// --- Inferred types ---

export type AppConfig = z.infer<typeof AppConfigSchema>;
export type LLMProviderConfig = z.infer<typeof LLMProviderSchema>;
export type LLMTier = z.infer<typeof LLMTierSchema>;
export type LLMConfig = z.infer<typeof LLMConfigSchema>;
export type AgentDefinition = z.infer<typeof AgentDefinitionSchema>;
export type StageInput = z.infer<typeof StageInputSchema>;
export type StageOutput = z.infer<typeof StageOutputSchema>;
export type StageOnReject = z.infer<typeof StageOnRejectSchema>;
export type StageDefinition = z.infer<typeof StageDefinitionSchema>;
export type WorkflowDefinition = z.infer<typeof WorkflowDefinitionSchema>;
export type WorkflowConfig = z.infer<typeof WorkflowConfigSchema>;
export type WebConfig = z.infer<typeof WebConfigSchema>;
export type EnvConfig = z.infer<typeof EnvConfigSchema>;
export type Config = z.infer<typeof FullConfigSchema>;
