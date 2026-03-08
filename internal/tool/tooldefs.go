package tool

import "github.com/dongowu/agentos/internal/adapters/llm"

// BuildToolDefs converts a slice of Tools into LLM ToolDef descriptors.
// If a tool implements SchemaAware, its schema is used; otherwise a generic
// object schema is provided as fallback.
func BuildToolDefs(tools []Tool) []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(tools))
	for _, t := range tools {
		def := llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  map[string]any{"type": "object"},
		}
		if sa, ok := t.(SchemaAware); ok {
			if s := sa.Schema(); s != nil {
				def.Parameters = s
			}
		}
		defs = append(defs, def)
	}
	return defs
}

// AllTools returns all registered tools as a slice.
func AllTools() []Tool {
	mu.RLock()
	defer mu.RUnlock()
	tools := make([]Tool, 0, len(registry))
	for _, t := range registry {
		tools = append(tools, t)
	}
	return tools
}
