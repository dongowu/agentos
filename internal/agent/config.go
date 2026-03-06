// Package agent provides Agent loading and execution for ClawOS.
package agent

import "fmt"

// Config is the agent.yaml structure. Agent is config, not code.
type Config struct {
	Name    string       `yaml:"name"`
	Model   string       `yaml:"model"`
	Memory  MemoryConfig `yaml:"memory"`
	Tools   []string     `yaml:"tools"`
	Workflow []string    `yaml:"workflow"`
}

// MemoryConfig configures the memory backend.
type MemoryConfig struct {
	Type string `yaml:"type"` // redis, vector
}

// Validate performs the minimal MVP validation for agent config.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("agent config is nil")
	}
	if c.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	return nil
}
