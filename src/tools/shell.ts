import { execFile } from 'node:child_process';
import type { Tool, ToolDefinition, ToolResult } from './types.js';

export class ShellTool implements Tool {
  private cwd: string;
  private timeoutMs: number;

  definition: ToolDefinition = {
    name: 'shell',
    description: 'Execute a shell command and capture stdout/stderr.',
    parameters: {
      type: 'object',
      required: ['command'],
      properties: {
        command: { type: 'string', description: 'The shell command to execute' },
        timeout: { type: 'number', description: 'Timeout in milliseconds' },
      },
    },
  };

  constructor(cwd: string, timeoutMs = 30_000) {
    this.cwd = cwd;
    this.timeoutMs = timeoutMs;
  }

  async execute(args: Record<string, unknown>): Promise<ToolResult> {
    const command = args.command as string;
    const timeout = (args.timeout as number) ?? this.timeoutMs;

    return new Promise((resolve) => {
      const isWin = process.platform === 'win32';
      const shell = isWin ? 'cmd.exe' : '/bin/sh';
      const flag = isWin ? '/c' : '-c';

      execFile(shell, [flag, command], { cwd: this.cwd, timeout }, (err, stdout, stderr) => {
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
