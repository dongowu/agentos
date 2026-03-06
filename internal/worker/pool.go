package worker

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

// Dialer creates an ExecutorClient for a given worker address.
type Dialer interface {
	Dial(ctx context.Context, addr string) (runtimeclient.ExecutorClient, error)
}

// Pool manages gRPC connections to multiple workers and routes executions.
// Connections are lazily established on first Execute and cached.
type Pool struct {
	mu       sync.RWMutex
	registry Registry
	dialer   Dialer
	clients  map[string]runtimeclient.ExecutorClient // workerID -> client
}

// NewPool creates a worker pool backed by the given registry and dialer.
func NewPool(registry Registry, dialer Dialer) *Pool {
	if dialer == nil {
		dialer = NewGRPCDialer()
	}
	return &Pool{
		registry: registry,
		dialer:   dialer,
		clients:  make(map[string]runtimeclient.ExecutorClient),
	}
}

// Execute routes an action to a specific worker. If no connection is cached,
// it looks up the worker address from the registry and dials lazily.
func (p *Pool) Execute(ctx context.Context, workerID, taskID string, action *taskdsl.Action) (*runtimeclient.ExecutionResult, error) {
	client, err := p.getOrDial(ctx, workerID)
	if err != nil {
		return nil, err
	}
	return client.ExecuteAction(ctx, taskID, action)
}

// SelectWorker picks the least-loaded available worker from the registry.
func (p *Pool) SelectWorker(ctx context.Context) (string, error) {
	available, err := p.registry.GetAvailable(ctx)
	if err != nil {
		return "", err
	}
	if len(available) == 0 {
		return "", errors.New("no available workers")
	}

	// Sort by active tasks ascending, then by ID for determinism.
	sort.Slice(available, func(i, j int) bool {
		if available[i].ActiveTasks != available[j].ActiveTasks {
			return available[i].ActiveTasks < available[j].ActiveTasks
		}
		return available[i].ID < available[j].ID
	})

	return available[0].ID, nil
}

// Disconnect closes a cached connection and removes it.
func (p *Pool) Disconnect(workerID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, exists := p.clients[workerID]
	if !exists {
		return errors.New("worker not connected: " + workerID)
	}

	if closer, ok := client.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	delete(p.clients, workerID)
	return nil
}

// ClientCount returns the number of cached client connections.
func (p *Pool) ClientCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clients)
}

// Close disconnects all cached workers.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, client := range p.clients {
		if closer, ok := client.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		delete(p.clients, id)
	}
	return nil
}

// getOrDial returns a cached client or establishes a new connection.
func (p *Pool) getOrDial(ctx context.Context, workerID string) (runtimeclient.ExecutorClient, error) {
	p.mu.RLock()
	client, exists := p.clients[workerID]
	p.mu.RUnlock()
	if exists {
		return client, nil
	}

	workers, err := p.registry.List(ctx)
	if err != nil {
		return nil, err
	}
	var addr string
	for _, w := range workers {
		if w.ID == workerID {
			addr = w.Addr
			break
		}
	}
	if addr == "" {
		return nil, errors.New("worker not found: " + workerID)
	}

	c, err := p.dialer.Dial(ctx, addr)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if existing, ok := p.clients[workerID]; ok {
		if closer, ok := c.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return existing, nil
	}
	p.clients[workerID] = c
	return c, nil
}
