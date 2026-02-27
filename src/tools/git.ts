import { execFile } from 'node:child_process';
import type { Tool, ToolDefinition, ToolResult } from './types.js';

export class GitTool implements Tool {
  private cwd: string;

  definition: ToolDefinition = {
    name: 'git',
    description: 'Run git operations: init, add, commit, createBranch, diff, log, status.',
    parameters: {
      type: 'object',
      required: ['operation'],
      properties: {
        operation: {
          type: 'string',
          enum: ['init', 'add', 'commit', 'createBranch', 'diff', 'log', 'status'],
        },
        args: {
          type: 'array',
          items: { type: 'string' },
          description: 'Additional arguments for the operation',
        },
        message: { type: 'string', description: 'Commit message (for commit)' },
        branch: { type: 'string', description: 'Branch name (for createBranch)' },
        files: {
          type: 'array',
          items: { type: 'string' },
          description: 'Files to add (for add)',
        },
      },
    },
  };

  constructor(cwd: string) {
    this.cwd = cwd;
  }

  async execute(args: Record<string, unknown>): Promise<ToolResult> {
    const op = args.operation as string;
    try {
      switch (op) {
        case 'init':
          return this.run(['init']);
        case 'add': {
          const files = (args.files as string[]) ?? ['.'];
          return this.run(['add', ...files]);
        }
        case 'commit': {
          const message = (args.message as string) ?? 'no message';
          return this.run(['commit', '-m', message]);
        }
        case 'createBranch': {
          const branch = args.branch as string;
          return this.run(['checkout', '-b', branch]);
        }
        case 'diff':
          return this.run(['diff', ...((args.args as string[]) ?? [])]);
        case 'log':
          return this.run(['log', '--oneline', '-20', ...((args.args as string[]) ?? [])]);
        case 'status':
          return this.run(['status', '--short']);
        default:
          return { success: false, output: '', error: `Unknown operation: ${op}` };
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      return { success: false, output: '', error: msg };
    }
  }

  private run(gitArgs: string[]): Promise<ToolResult> {
    return new Promise((resolve) => {
      execFile('git', gitArgs, { cwd: this.cwd }, (err, stdout, stderr) => {
        const output = [stdout, stderr].filter(Boolean).join('\n');
        if (err) {
          resolve({ success: false, output, error: err.message });
        } else {
          resolve({ success: true, output });
        }
      });
    });
  }
}