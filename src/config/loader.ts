import { existsSync, readFileSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { parse as parseYaml } from "yaml";
import { type Config, FullConfigSchema } from "./schema.js";

/**
 * Parse a .env file and load into process.env (does not override existing vars).
 */
function loadDotEnv(dir: string): void {
  const envPath = join(dir, ".env");
  if (!existsSync(envPath)) return;
  const content = readFileSync(envPath, "utf-8");
  for (const line of content.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const eqIdx = trimmed.indexOf("=");
    if (eqIdx === -1) continue;
    const key = trimmed.slice(0, eqIdx).trim();
    let value = trimmed.slice(eqIdx + 1).trim();
    // Strip surrounding quotes
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    if (!process.env[key]) {
      process.env[key] = value;
    }
  }
}

/**
 * Deep-merge `source` into `target` (mutates target).
 * Arrays are replaced, not concatenated.
 */
function deepMerge(target: Record<string, unknown>, source: Record<string, unknown>): Record<string, unknown> {
  for (const key of Object.keys(source)) {
    const sv = source[key];
    const tv = target[key];
    if (sv && tv && typeof sv === "object" && typeof tv === "object" && !Array.isArray(sv)) {
      deepMerge(tv as Record<string, unknown>, sv as Record<string, unknown>);
    } else {
      target[key] = sv;
    }
  }
  return target;
}

/**
 * Load, merge, validate, and return the full config.
 * @param configDir - path to the directory containing YAML files (defaults to `<projectRoot>/config`)
 */
export function loadConfig(configDir?: string): Config {
  const dir = configDir ? resolve(configDir) : resolve(process.cwd(), "config");

  // Load .env from project root (parent of config dir)
  loadDotEnv(resolve(dir, ".."));

  // Read every .yaml / .yml in the config directory and merge them
  const files = readdirSync(dir).filter((f) => /\.ya?ml$/i.test(f)).sort();
  let merged: Record<string, unknown> = {};
  for (const file of files) {
    const raw = readFileSync(join(dir, file), "utf-8");
    const parsed = parseYaml(raw) as Record<string, unknown> | null;
    if (parsed) {
      merged = deepMerge(merged, parsed);
    }
  }

  // Apply environment-variable overrides
  const env = process.env;
  if (env.LOG_LEVEL) {
    (merged.app as Record<string, unknown>).logLevel = env.LOG_LEVEL;
  }
  if (env.DATA_DIR) {
    (merged.app as Record<string, unknown>).dataDir = env.DATA_DIR;
  }
  if (env.WEB_PORT) {
    (merged.web as Record<string, unknown>).port = Number(env.WEB_PORT);
  }

  // Collect API keys / provider URLs into `env` field
  merged.env = {
    anthropicApiKey: env.ANTHROPIC_API_KEY,
    anthropicBaseUrl: env.ANTHROPIC_BASE_URL,
    openaiApiKey: env.OPENAI_API_KEY,
    openaiBaseUrl: env.OPENAI_BASE_URL,
    openaiWireApi: env.OPENAI_WIRE_API,
    deepseekApiKey: env.DEEPSEEK_API_KEY,
    deepseekBaseUrl: env.DEEPSEEK_BASE_URL,
  };

  return FullConfigSchema.parse(merged);
}
