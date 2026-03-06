package agent

import (
	"fmt"
	"path/filepath"

	"github.com/dongowu/agentos/internal/memory"
	"github.com/dongowu/agentos/internal/tool"
)

// AgentRuntime wires an agent config to resolved tools and memory at runtime.
type AgentRuntime struct {
	Config     *Config
	Tools      []tool.Tool
	MemoryProv memory.Provider
}

// NewRuntime creates an AgentRuntime from a validated Config.
// It resolves tool references from the global tool registry.
func NewRuntime(cfg *Config) (*AgentRuntime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	rt := &AgentRuntime{Config: cfg}

	// Resolve tools from registry (best-effort: skip unregistered tools)
	for _, name := range cfg.Tools {
		if t := tool.Get(name); t != nil {
			rt.Tools = append(rt.Tools, t)
		}
	}

	return rt, nil
}

// CheckPolicy determines if a tool is permitted by the agent's policy.
// Deny rules take precedence over allow rules.
// An empty policy (no allow/deny) permits everything.
func (r *AgentRuntime) CheckPolicy(toolName string) error {
	policy := r.Config.Policy

	// Empty policy = open access
	if len(policy.Allow) == 0 && len(policy.Deny) == 0 {
		return nil
	}

	// Check deny first (deny takes precedence)
	for _, pattern := range policy.Deny {
		if matched, _ := filepath.Match(pattern, toolName); matched {
			return fmt.Errorf("tool %q denied by policy pattern %q", toolName, pattern)
		}
	}

	// If allow list is empty, allow everything not denied
	if len(policy.Allow) == 0 {
		return nil
	}

	// Check allow
	for _, pattern := range policy.Allow {
		if matched, _ := filepath.Match(pattern, toolName); matched {
			return nil
		}
	}

	return fmt.Errorf("tool %q not matched by any allow policy", toolName)
}

// AvailableTools returns the names of tools declared in this agent's config.
func (r *AgentRuntime) AvailableTools() []string {
	return r.Config.Tools
}
