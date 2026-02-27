import { Command } from 'commander';
import ora from 'ora';
import {
  formatPipelineStatus,
  formatCostSummary,
  formatProjectList,
} from './formatters.js';

const program = new Command();

program
  .name('orch')
  .description('AI Orchestrator CLI')
  .version('0.1.0');

// ── run ───────────────────────────────────────────────────────────

program
  .command('run <requirement>')
  .description('Start a new project pipeline')
  .action(async (requirement: string) => {
    const spinner = ora('Starting pipeline...').start();
    try {
      const { bootstrap } = await import('../index.js');
      const system = await bootstrap();
      spinner.text = 'Running pipeline...';
      const result = await system.pipelineEngine.start(requirement, 'mvp');
      spinner.succeed('Pipeline completed');
      console.log(result);
    } catch (err) {
      spinner.fail('Pipeline failed');
      console.error(err instanceof Error ? err.message : err);
      process.exitCode = 1;
    }
  });

// ── status ────────────────────────────────────────────────────────

program
  .command('status [pipeline-id]')
  .description('Show pipeline status')
  .action(async (pipelineId?: string) => {
    try {
      const { bootstrap } = await import('../index.js');
      const system = await bootstrap();
      const pipeline = pipelineId
        ? system.repositories.pipelines.findById(pipelineId)
        : undefined;
      if (!pipeline) {
        console.log('No pipeline found.');
        return;
      }
      console.log(formatPipelineStatus(pipeline, []));
    } catch (err) {
      console.error(err instanceof Error ? err.message : err);
      process.exitCode = 1;
    }
  });

// ── resume ────────────────────────────────────────────────────────

program
  .command('resume <pipeline-id>')
  .description('Resume a paused pipeline')
  .action(async (pipelineId: string) => {
    const spinner = ora('Resuming pipeline...').start();
    try {
      const { bootstrap } = await import('../index.js');
      const system = await bootstrap();
      await system.pipelineEngine.resume(pipelineId);
      spinner.succeed('Pipeline resumed');
    } catch (err) {
      spinner.fail('Resume failed');
      console.error(err instanceof Error ? err.message : err);
      process.exitCode = 1;
    }
  });

// ── approve ───────────────────────────────────────────────────────

program
  .command('approve <decision-id>')
  .description('Approve a human gate decision')
  .action(async (decisionId: string) => {
    try {
      const { bootstrap } = await import('../index.js');
      const system = await bootstrap();
      await system.humanGate.approve(decisionId);
      console.log(`Decision ${decisionId} approved.`);
    } catch (err) {
      console.error(err instanceof Error ? err.message : err);
      process.exitCode = 1;
    }
  });

// ── reject ────────────────────────────────────────────────────────

program
  .command('reject <decision-id> [reason]')
  .description('Reject a human gate decision')
  .action(async (decisionId: string, reason?: string) => {
    try {
      const { bootstrap } = await import('../index.js');
      const system = await bootstrap();
      await system.humanGate.reject(decisionId, reason);
      console.log(`Decision ${decisionId} rejected.`);
    } catch (err) {
      console.error(err instanceof Error ? err.message : err);
      process.exitCode = 1;
    }
  });

// ── history ───────────────────────────────────────────────────────

program
  .command('history')
  .description('List past projects')
  .action(async () => {
    try {
      const { bootstrap } = await import('../index.js');
      const system = await bootstrap();
      const projects = system.repositories.projects.findAll();
      console.log(formatProjectList(projects));
    } catch (err) {
      console.error(err instanceof Error ? err.message : err);
      process.exitCode = 1;
    }
  });

// ── cost ──────────────────────────────────────────────────────────

program
  .command('cost [pipeline-id]')
  .description('Show token usage and cost')
  .action(async (pipelineId?: string) => {
    try {
      const { bootstrap } = await import('../index.js');
      const system = await bootstrap();
      const summary = system.tokenTracker.getSummary(pipelineId);
      console.log(formatCostSummary(summary));
    } catch (err) {
      console.error(err instanceof Error ? err.message : err);
      process.exitCode = 1;
    }
  });

program.parse();
