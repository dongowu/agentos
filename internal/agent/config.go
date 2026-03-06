// Package agent provides Agent loading and execution for ClawOS.
package agent

import "fmt"

// Config is the agent.yaml structure. Agent is config, not code.
type Config struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Model       string       `yaml:"model"`
	Memory      MemoryConfig `yaml:"memory"`
	Tools       []string     `yaml:"tools"`
	Policy      PolicyConfig `yaml:"policy"`
	Workflow    []string     `yaml:"workflow"`
}

// MemoryConfig configures the memory backend.
type MemoryConfig struct {
	Type string `yaml:"type"` // redis, vector
	TTL  int    `yaml:"ttl"`  // seconds, 0 means no expiry
}

// PolicyConfig defines allow/deny patterns for tool access control.
type PolicyConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// Validate performs validation for agent config.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("agent config is nil")
	}
	if c.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if c.Model == "" {
		return fmt.Errorf("agent model is required")
	}
	if c.Memory.TTL < 0 {
		return fmt.Errorf("memory ttl must be non-negative")
	}
	return nil
}
