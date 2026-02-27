import type { Tool, ToolDefinition } from './types.js';

export class ToolRegistry {
  private tools = new Map<string, Tool>();
  private capabilityMap = new Map<string, Set<string>>();

  register(capability: string, tool: Tool): void {
    this.tools.set(tool.definition.name, tool);
    let set = this.capabilityMap.get(capability);
    if (!set) {
      set = new Set();
      this.capabilityMap.set(capability, set);
    }
    set.add(tool.definition.name);
  }

  getTools(capabilities: string[]): Tool[] {
    const names = new Set<string>();
    for (const cap of capabilities) {
      const set = this.capabilityMap.get(cap);
      if (set) {
        for (const name of set) names.add(name);
      }
    }
    return [...names].map((n) => this.tools.get(n)!).filter(Boolean);
  }

  getTool(name: string): Tool | undefined {
    return this.tools.get(name);
  }

  getDefinitions(capabilities: string[]): ToolDefinition[] {
    return this.getTools(capabilities).map((t) => t.definition);
  }
}
