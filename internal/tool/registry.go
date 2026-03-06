package tool

import "sync"

var (
	registry = make(map[string]Tool)
	mu       sync.RWMutex
)

// Register registers a tool. Panics if name is already registered.
func Register(t Tool) {
	mu.Lock()
	defer mu.Unlock()
	name := t.Name()
	if _, ok := registry[name]; ok {
		panic("tool: duplicate registration of " + name)
	}
	registry[name] = t
}

// Get returns the tool by name, or nil if not found.
func Get(name string) Tool {
	mu.RLock()
	defer mu.RUnlock()
	return registry[name]
}

// List returns all registered tool names.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}
