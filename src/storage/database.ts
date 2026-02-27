import fs from "node:fs";
import path from "node:path";
import Database from "better-sqlite3";
import { drizzle, type BetterSQLite3Database } from "drizzle-orm/better-sqlite3";
import * as schema from "./schema.js";

const CREATE_TABLES_SQL = `
CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  requirement TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  working_dir TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pipelines (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id),
  definition_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  current_stage TEXT,
  state_json TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  parent_id TEXT,
  type TEXT NOT NULL,
  routing TEXT NOT NULL DEFAULT 'unicast',
  from_role TEXT NOT NULL,
  to_role TEXT,
  subject TEXT,
  body TEXT NOT NULL,
  metadata_json TEXT,
  timestamp TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS conversations (
  id TEXT PRIMARY KEY,
  pipeline_id TEXT NOT NULL,
  stage_id TEXT,
  topic TEXT NOT NULL,
  participants_json TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  round_count INTEGER NOT NULL DEFAULT 0,
  max_rounds INTEGER NOT NULL DEFAULT 5,
  created_at TEXT NOT NULL,
  resolved_at TEXT
);

CREATE TABLE IF NOT EXISTS artifacts (
  id TEXT PRIMARY KEY,
  pipeline_id TEXT NOT NULL,
  stage_type TEXT NOT NULL,
  artifact_type TEXT NOT NULL,
  name TEXT NOT NULL,
  file_path TEXT NOT NULL,
  content_hash TEXT,
  version INTEGER NOT NULL DEFAULT 1,
  created_by TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS token_usage (
  id TEXT PRIMARY KEY,
  pipeline_id TEXT,
  agent_role TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  input_tokens INTEGER NOT NULL,
  output_tokens INTEGER NOT NULL,
  cost_usd REAL NOT NULL DEFAULT 0,
  timestamp TEXT NOT NULL
);
`;

export type AppDatabase = BetterSQLite3Database<typeof schema>;

export function initDatabase(dbPath?: string): AppDatabase {
  const resolvedPath = dbPath ?? path.resolve("data", "orchestrator.db");
  const dir = path.dirname(resolvedPath);
  fs.mkdirSync(dir, { recursive: true });

  const sqlite = new Database(resolvedPath);
  sqlite.pragma("journal_mode = WAL");
  sqlite.exec(CREATE_TABLES_SQL);

  return drizzle(sqlite, { schema });
}
