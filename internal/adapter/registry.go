package adapter

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/dongowu/agentos/internal/messaging"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/config"
)

// EventBusFactory creates an EventBus from messaging config.
type EventBusFactory func(cfg config.MessagingConfig) (messaging.EventBus, error)

// TaskRepoFactory creates a TaskRepository from persistence config.
type TaskRepoFactory func(ctx context.Context, cfg config.PersistenceConfig) (persistence.TaskRepository, error)

// AuditLogStoreFactory creates an AuditLogStore from persistence config.
type AuditLogStoreFactory func(ctx context.Context, cfg config.PersistenceConfig) (persistence.AuditLogStore, error)

var (
	mu             sync.RWMutex
	busFactories   = map[string]EventBusFactory{}
	repoFactories  = map[string]TaskRepoFactory{}
	auditFactories = map[string]AuditLogStoreFactory{}
)

// RegisterEventBus registers an EventBus factory under the given name.
// Panics if the name is already registered.
func RegisterEventBus(name string, factory EventBusFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := busFactories[name]; dup {
		panic(fmt.Sprintf("adapter: duplicate EventBus registration: %s", name))
	}
	busFactories[name] = factory
}

// RegisterTaskRepo registers a TaskRepository factory under the given name.
// Panics if the name is already registered.
func RegisterTaskRepo(name string, factory TaskRepoFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := repoFactories[name]; dup {
		panic(fmt.Sprintf("adapter: duplicate TaskRepo registration: %s", name))
	}
	repoFactories[name] = factory
}

// RegisterAuditLogStore registers an AuditLogStore factory under the given name.
// Panics if the name is already registered.
func RegisterAuditLogStore(name string, factory AuditLogStoreFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := auditFactories[name]; dup {
		panic(fmt.Sprintf("adapter: duplicate AuditLogStore registration: %s", name))
	}
	auditFactories[name] = factory
}

// NewEventBus looks up the registered factory for cfg.Provider and creates an EventBus.
func NewEventBus(cfg config.MessagingConfig) (messaging.EventBus, error) {
	mu.RLock()
	factory, ok := busFactories[cfg.Provider]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("adapter: unknown EventBus provider %q (registered: %v)", cfg.Provider, RegisteredEventBusNames())
	}
	return factory(cfg)
}

// NewTaskRepo looks up the registered factory for cfg.Provider and creates a TaskRepository.
func NewTaskRepo(ctx context.Context, cfg config.PersistenceConfig) (persistence.TaskRepository, error) {
	mu.RLock()
	factory, ok := repoFactories[cfg.Provider]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("adapter: unknown TaskRepo provider %q (registered: %v)", cfg.Provider, RegisteredTaskRepoNames())
	}
	return factory(ctx, cfg)
}

// NewAuditLogStore looks up the registered factory for cfg.Provider and creates an AuditLogStore.
func NewAuditLogStore(ctx context.Context, cfg config.PersistenceConfig) (persistence.AuditLogStore, error) {
	mu.RLock()
	factory, ok := auditFactories[cfg.Provider]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("adapter: unknown AuditLogStore provider %q (registered: %v)", cfg.Provider, RegisteredAuditLogStoreNames())
	}
	return factory(ctx, cfg)
}

// RegisteredEventBusNames returns sorted names of all registered EventBus factories.
func RegisteredEventBusNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(busFactories))
	for name := range busFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RegisteredTaskRepoNames returns sorted names of all registered TaskRepo factories.
func RegisteredTaskRepoNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(repoFactories))
	for name := range repoFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RegisteredAuditLogStoreNames returns sorted names of all registered AuditLogStore factories.
func RegisteredAuditLogStoreNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(auditFactories))
	for name := range auditFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
