import { sqliteTable, text, integer, real } from "drizzle-orm/sqlite-core";

export const projects = sqliteTable("projects", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  requirement: text("requirement").notNull(),
  status: text("status").notNull().default("pending"),
  working_dir: text("working_dir"),
  created_at: text("created_at").notNull(),
  updated_at: text("updated_at").notNull(),
});

export const pipelines = sqliteTable("pipelines", {
  id: text("id").primaryKey(),
  project_id: text("project_id")
    .notNull()
    .references(() => projects.id),
  definition_id: text("definition_id").notNull(),
  status: text("status").notNull().default("pending"),
  current_stage: text("current_stage"),
  state_json: text("state_json"),
  created_at: text("created_at").notNull(),
  updated_at: text("updated_at").notNull(),
});

export const messages = sqliteTable("messages", {
  id: text("id").primaryKey(),
  conversation_id: text("conversation_id").notNull(),
  parent_id: text("parent_id"),
  type: text("type").notNull(),
  routing: text("routing").notNull().default("unicast"),
  from_role: text("from_role").notNull(),
  to_role: text("to_role"),
  subject: text("subject"),
  body: text("body").notNull(),
  metadata_json: text("metadata_json"),
  timestamp: text("timestamp").notNull(),
});

export const conversations = sqliteTable("conversations", {
  id: text("id").primaryKey(),
  pipeline_id: text("pipeline_id").notNull(),
  stage_id: text("stage_id"),
  topic: text("topic").notNull(),
  participants_json: text("participants_json").notNull(),
  status: text("status").notNull().default("active"),
  round_count: integer("round_count").notNull().default(0),
  max_rounds: integer("max_rounds").notNull().default(5),
  created_at: text("created_at").notNull(),
  resolved_at: text("resolved_at"),
});

export const artifacts = sqliteTable("artifacts", {
  id: text("id").primaryKey(),
  pipeline_id: text("pipeline_id").notNull(),
  stage_type: text("stage_type").notNull(),
  artifact_type: text("artifact_type").notNull(),
  name: text("name").notNull(),
  file_path: text("file_path").notNull(),
  content_hash: text("content_hash"),
  version: integer("version").notNull().default(1),
  created_by: text("created_by").notNull(),
  created_at: text("created_at").notNull(),
});

export const tokenUsage = sqliteTable("token_usage", {
  id: text("id").primaryKey(),
  pipeline_id: text("pipeline_id"),
  agent_role: text("agent_role").notNull(),
  provider: text("provider").notNull(),
  model: text("model").notNull(),
  input_tokens: integer("input_tokens").notNull(),
  output_tokens: integer("output_tokens").notNull(),
  cost_usd: real("cost_usd").notNull().default(0),
  timestamp: text("timestamp").notNull(),
});
