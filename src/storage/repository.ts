import { and, eq, sql } from "drizzle-orm";
import type { AppDatabase } from "./database.js";
import {
  projects,
  pipelines,
  messages,
  conversations,
  artifacts,
  tokenUsage,
} from "./schema.js";

// ── ProjectRepository ──────────────────────────────────────────────

export class ProjectRepository {
  constructor(private db: AppDatabase) {}

  create(data: typeof projects.$inferInsert) {
    return this.db.insert(projects).values(data).returning().get();
  }

  findById(id: string) {
    return this.db.select().from(projects).where(eq(projects.id, id)).get();
  }

  findAll() {
    return this.db.select().from(projects).all();
  }

  updateStatus(id: string, status: string) {
    return this.db
      .update(projects)
      .set({ status, updated_at: new Date().toISOString() })
      .where(eq(projects.id, id))
      .returning()
      .get();
  }
}

// ── PipelineRepository ─────────────────────────────────────────────

export class PipelineRepository {
  constructor(private db: AppDatabase) {}

  create(data: typeof pipelines.$inferInsert) {
    return this.db.insert(pipelines).values(data).returning().get();
  }

  findById(id: string) {
    return this.db.select().from(pipelines).where(eq(pipelines.id, id)).get();
  }

  findByProjectId(projectId: string) {
    return this.db
      .select()
      .from(pipelines)
      .where(eq(pipelines.project_id, projectId))
      .all();
  }

  updateStatus(id: string, status: string) {
    return this.db
      .update(pipelines)
      .set({ status, updated_at: new Date().toISOString() })
      .where(eq(pipelines.id, id))
      .returning()
      .get();
  }

  updateStage(id: string, stage: string) {
    return this.db
      .update(pipelines)
      .set({ current_stage: stage, updated_at: new Date().toISOString() })
      .where(eq(pipelines.id, id))
      .returning()
      .get();
  }

  updateState(id: string, stateJson: string) {
    return this.db
      .update(pipelines)
      .set({ state_json: stateJson, updated_at: new Date().toISOString() })
      .where(eq(pipelines.id, id))
      .returning()
      .get();
  }
}

// ── MessageRepository ──────────────────────────────────────────────

export class MessageRepository {
  constructor(private db: AppDatabase) {}

  create(data: typeof messages.$inferInsert) {
    return this.db.insert(messages).values(data).returning().get();
  }

  findByConversationId(conversationId: string) {
    return this.db
      .select()
      .from(messages)
      .where(eq(messages.conversation_id, conversationId))
      .all();
  }

  findByPipelineId(pipelineId: string) {
    return this.db
      .select()
      .from(messages)
      .innerJoin(
        conversations,
        eq(messages.conversation_id, conversations.id),
      )
      .where(eq(conversations.pipeline_id, pipelineId))
      .all();
  }
}

// ── ConversationRepository ─────────────────────────────────────────

export class ConversationRepository {
  constructor(private db: AppDatabase) {}

  create(data: typeof conversations.$inferInsert) {
    return this.db.insert(conversations).values(data).returning().get();
  }

  findById(id: string) {
    return this.db
      .select()
      .from(conversations)
      .where(eq(conversations.id, id))
      .get();
  }

  findByPipelineId(pipelineId: string) {
    return this.db
      .select()
      .from(conversations)
      .where(eq(conversations.pipeline_id, pipelineId))
      .all();
  }
  updateStatus(id: string, status: string) {
    return this.db
      .update(conversations)
      .set({ status })
      .where(eq(conversations.id, id))
      .returning()
      .get();
  }

  incrementRound(id: string) {
    return this.db
      .update(conversations)
      .set({ round_count: sql`${conversations.round_count} + 1` })
      .where(eq(conversations.id, id))
      .returning()
      .get();
  }
}

// ── ArtifactRepository ─────────────────────────────────────────────

export class ArtifactRepository {
  constructor(private db: AppDatabase) {}

  create(data: typeof artifacts.$inferInsert) {
    return this.db.insert(artifacts).values(data).returning().get();
  }

  findByPipelineId(pipelineId: string) {
    return this.db
      .select()
      .from(artifacts)
      .where(eq(artifacts.pipeline_id, pipelineId))
      .all();
  }

  findByStageType(pipelineId: string, stageType: string) {
    return this.db
      .select()
      .from(artifacts)
      .where(
        and(
          eq(artifacts.pipeline_id, pipelineId),
          eq(artifacts.stage_type, stageType),
        ),
      )
      .all();
  }
}
// ── TokenUsageRepository ───────────────────────────────────────────

export class TokenUsageRepository {
  constructor(private db: AppDatabase) {}

  create(data: typeof tokenUsage.$inferInsert) {
    return this.db.insert(tokenUsage).values(data).returning().get();
  }

  sumByPipelineId(pipelineId: string) {
    return this.db
      .select({
        totalInput: sql<number>`sum(${tokenUsage.input_tokens})`,
        totalOutput: sql<number>`sum(${tokenUsage.output_tokens})`,
        totalCost: sql<number>`sum(${tokenUsage.cost_usd})`,
      })
      .from(tokenUsage)
      .where(eq(tokenUsage.pipeline_id, pipelineId))
      .get();
  }

  sumByProvider(provider: string) {
    return this.db
      .select({
        totalInput: sql<number>`sum(${tokenUsage.input_tokens})`,
        totalOutput: sql<number>`sum(${tokenUsage.output_tokens})`,
        totalCost: sql<number>`sum(${tokenUsage.cost_usd})`,
      })
      .from(tokenUsage)
      .where(eq(tokenUsage.provider, provider))
      .get();
  }

  sumAll() {
    return this.db
      .select({
        totalInput: sql<number>`sum(${tokenUsage.input_tokens})`,
        totalOutput: sql<number>`sum(${tokenUsage.output_tokens})`,
        totalCost: sql<number>`sum(${tokenUsage.cost_usd})`,
      })
      .from(tokenUsage)
      .get();
  }
}