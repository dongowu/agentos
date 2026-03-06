package agent

import (
	"os"
	"path/filepath"
	"sort"
)

// Manager manages a collection of agent runtimes loaded from config files.
type Manager struct {
	agents map[string]*AgentRuntime
}

// NewManager creates a new Manager.
func NewManager() *Manager {
	return &Manager{agents: make(map[string]*AgentRuntime)}
}

// LoadFromDir loads all *.yaml files from a directory as agent configs.
func (m *Manager) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		cfg, err := Load(path)
		if err != nil {
			return err
		}
		rt, err := NewRuntime(cfg)
		if err != nil {
			return err
		}
		m.agents[cfg.Name] = rt
	}
	return nil
}

// Get returns the AgentRuntime for the given agent name, or nil if not found.
func (m *Manager) Get(name string) *AgentRuntime {
	return m.agents[name]
}

// List returns all loaded agent names sorted alphabetically.
func (m *Manager) List() []string {
	names := make([]string, 0, len(m.agents))
	for n := range m.agents {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
