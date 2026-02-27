import * as fs from 'node:fs/promises';
import * as path from 'node:path';
import type { Tool, ToolDefinition, ToolResult } from './types.js';

export class FilesystemTool implements Tool {
  private baseDir: string;

  definition: ToolDefinition = {
    name: 'filesystem',
    description: 'Read, write, list, create, and delete files within the working directory.',
    parameters: {
      type: 'object',
      required: ['operation'],
      properties: {
        operation: {
          type: 'string',
          enum: ['readFile', 'writeFile', 'listDirectory', 'createDirectory', 'deleteFile'],
        },
        path: { type: 'string', description: 'Relative path within the working directory' },
        content: { type: 'string', description: 'File content (for writeFile)' },
      },
    },
  };

  constructor(baseDir: string) {
    this.baseDir = path.resolve(baseDir);
  }

  async execute(args: Record<string, unknown>): Promise<ToolResult> {
    const op = args.operation as string;
    const relPath = (args.path as string) ?? '';
    const resolved = this.resolveSafe(relPath);
    if (!resolved) {
      return { success: false, output: '', error: 'Path traversal denied' };
    }

    try {
      switch (op) {
        case 'readFile': {
          const content = await fs.readFile(resolved, 'utf-8');
          return { success: true, output: content };
        }
        case 'writeFile': {
          const content = args.content as string;
          await fs.mkdir(path.dirname(resolved), { recursive: true });
          await fs.writeFile(resolved, content, 'utf-8');
          return { success: true, output: `Wrote ${resolved}` };
        }
        case 'listDirectory': {
          const entries = await fs.readdir(resolved, { withFileTypes: true });
          const listing = entries.map((e) => `${e.isDirectory() ? 'd' : 'f'} ${e.name}`).join('\n');
          return { success: true, output: listing };
        }
        case 'createDirectory': {
          await fs.mkdir(resolved, { recursive: true });
          return { success: true, output: `Created ${resolved}` };
        }
        case 'deleteFile': {
          await fs.unlink(resolved);
          return { success: true, output: `Deleted ${resolved}` };
        }
        default:
          return { success: false, output: '', error: `Unknown operation: ${op}` };
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      return { success: false, output: '', error: msg };
    }
  }

  private resolveSafe(relPath: string): string | null {
    const resolved = path.resolve(this.baseDir, relPath);
    if (!resolved.startsWith(this.baseDir)) {
      return null;
    }
    return resolved;
  }
}
