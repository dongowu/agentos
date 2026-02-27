import type { TokenSummary } from '../llm/token-tracker.js';

// ── Pipeline Status ───────────────────────────────────────────────

interface PipelineInfo {
  id: string;
  status: string;
  current_stage: string | null;
  definition_id: string;
  created_at: string;
  updated_at: string;
}

interface StageInfo {
  id: string;
  type: string;
  status: string;
}

export function formatPipelineStatus(pipeline: PipelineInfo, stages: StageInfo[]): string {
  const lines: string[] = [];
  lines.push(`Pipeline: ${pipeline.id}`);
  lines.push(`Status:   ${pipeline.status}`);
  lines.push(`Workflow: ${pipeline.definition_id}`);
  lines.push(`Stage:    ${pipeline.current_stage ?? '(none)'}`);
  lines.push(`Created:  ${pipeline.created_at}`);
  lines.push(`Updated:  ${pipeline.updated_at}`);

  if (stages.length > 0) {
    lines.push('');
    lines.push('Stages:');
    for (const stage of stages) {
      const marker = stage.status === 'completed' ? '[x]'
        : stage.status === 'running' ? '[>]'
        : '[ ]';
      lines.push(`  ${marker} ${stage.id} (${stage.type}) - ${stage.status}`);
    }
  }

  return lines.join('\n');
}

// ── Cost Summary ──────────────────────────────────────────────────

export function formatCostSummary(usage: TokenSummary): string {
  const lines: string[] = [];
  const header = padRow('Source', 'Input Tokens', 'Output Tokens', 'Est. Cost');
  const sep = '-'.repeat(header.length);

  lines.push(header);
  lines.push(sep);

  for (const [provider, data] of Object.entries(usage.byProvider)) {
    lines.push(padRow(
      provider,
      data.inputTokens.toLocaleString(),
      data.outputTokens.toLocaleString(),
      `$${data.estimatedCost.toFixed(4)}`,
    ));
  }

  lines.push(sep);
  lines.push(padRow(
    'TOTAL',
    usage.totalInputTokens.toLocaleString(),
    usage.totalOutputTokens.toLocaleString(),
    `$${usage.totalEstimatedCost.toFixed(4)}`,
  ));

  return lines.join('\n');
}

// ── Project List ──────────────────────────────────────────────────

interface ProjectInfo {
  id: string;
  name: string;
  status: string;
  created_at: string;
}

export function formatProjectList(projects: ProjectInfo[]): string {
  if (projects.length === 0) return 'No projects found.';

  const lines: string[] = [];
  lines.push(padRow('ID', 'Name', 'Status', 'Created'));
  lines.push('-'.repeat(80));

  for (const p of projects) {
    lines.push(padRow(
      p.id.slice(0, 12),
      p.name.slice(0, 30),
      p.status,
      p.created_at.slice(0, 10),
    ));
  }

  return lines.join('\n');
}

// ── Decision List ─────────────────────────────────────────────────

interface DecisionInfo {
  id: string;
  pipeline_id: string;
  stage_id: string;
  description: string;
  status: string;
}

export function formatDecisionList(decisions: DecisionInfo[]): string {
  if (decisions.length === 0) return 'No pending decisions.';

  const lines: string[] = [];
  lines.push(padRow('ID', 'Pipeline', 'Stage', 'Status'));
  lines.push('-'.repeat(72));

  for (const d of decisions) {
    lines.push(padRow(
      d.id.slice(0, 12),
      d.pipeline_id.slice(0, 12),
      d.stage_id,
      d.status,
    ));
    if (d.description) {
      lines.push(`  ${d.description}`);
    }
  }

  return lines.join('\n');
}

// ── Helpers ───────────────────────────────────────────────────────

function padRow(...cols: string[]): string {
  const widths = [16, 20, 20, 16];
  return cols.map((c, i) => c.padEnd(widths[i] ?? 16)).join('');
}
