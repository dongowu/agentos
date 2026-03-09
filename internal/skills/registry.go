package skills

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// FieldDefinition describes one payload field accepted by a skill.
type FieldDefinition struct {
	Names    []string
	Type     string
	Required bool
}

// Definition describes how an action kind maps to a runtime profile.
type Definition struct {
	Name           string
	RuntimeProfile string
	Fields         []FieldDefinition
}

// Registry stores skill definitions by action kind.
type Registry struct {
	mu   sync.RWMutex
	defs map[string]Definition
}

// NewRegistry returns an empty registry and optionally preloads definitions.
func NewRegistry(defs ...Definition) *Registry {
	r := &Registry{defs: make(map[string]Definition, len(defs))}
	for _, def := range defs {
		if err := r.Register(def); err != nil {
			panic(err)
		}
	}
	return r
}

// Register adds a definition to the registry.
func (r *Registry) Register(def Definition) error {
	if r == nil {
		return fmt.Errorf("skills: nil registry")
	}
	name := strings.TrimSpace(def.Name)
	if name == "" {
		return fmt.Errorf("skills: definition name required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.defs[name]; exists {
		return fmt.Errorf("skills: duplicate definition %q", name)
	}
	def.Name = name
	r.defs[name] = def
	return nil
}

// Get returns a definition by action kind.
func (r *Registry) Get(name string) (Definition, bool) {
	if r == nil {
		return Definition{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.defs[strings.TrimSpace(name)]
	return def, ok
}

// List returns all registered action kinds.
func (r *Registry) List() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.defs))
	for name := range r.defs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var defaultRegistry = NewRegistry(builtinDefinitions()...)

// DefaultRegistry returns the process-wide builtin registry.
func DefaultRegistry() *Registry { return defaultRegistry }
