package worker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MemoryRegistry is a thread-safe, in-memory implementation of Registry.
type MemoryRegistry struct {
	mu      sync.RWMutex
	workers map[string]*WorkerInfo
	timeout time.Duration
}

// NewMemoryRegistry creates a registry that considers workers stale after timeout.
func NewMemoryRegistry(heartbeatTimeout time.Duration) *MemoryRegistry {
	return &MemoryRegistry{
		workers: make(map[string]*WorkerInfo),
		timeout: heartbeatTimeout,
	}
}

func (r *MemoryRegistry) Register(_ context.Context, info WorkerInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, exists := r.workers[info.ID]; exists {
		existing.Addr = info.Addr
		existing.Capabilities = append([]string(nil), info.Capabilities...)
		existing.MaxTasks = info.MaxTasks
		existing.ActiveTasks = 0
		existing.Status = StatusOnline
		existing.LastHeartbeat = time.Now()
		return nil
	}

	info.Status = StatusOnline
	info.LastHeartbeat = time.Now()
	r.workers[info.ID] = &info
	return nil
}

func (r *MemoryRegistry) Deregister(_ context.Context, workerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.workers[workerID]; !exists {
		return errors.New("worker not found: " + workerID)
	}
	delete(r.workers, workerID)
	return nil
}

func (r *MemoryRegistry) Heartbeat(_ context.Context, workerID string, activeTasks int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	w, exists := r.workers[workerID]
	if !exists {
		return errors.New("worker not found: " + workerID)
	}

	w.LastHeartbeat = time.Now()
	w.ActiveTasks = activeTasks

	if activeTasks >= w.MaxTasks {
		w.Status = StatusBusy
	} else {
		w.Status = StatusOnline
	}
	return nil
}

func (r *MemoryRegistry) List(_ context.Context) ([]WorkerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]WorkerInfo, 0, len(r.workers))
	for _, w := range r.workers {
		out = append(out, *w)
	}
	return out, nil
}

func (r *MemoryRegistry) GetAvailable(_ context.Context) ([]WorkerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []WorkerInfo
	for _, w := range r.workers {
		if w.Status == StatusOnline && w.ActiveTasks < w.MaxTasks {
			out = append(out, *w)
		}
	}
	return out, nil
}

// MarkOffline sets a worker's status to offline. Used by HealthMonitor.
func (r *MemoryRegistry) MarkOffline(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if w, ok := r.workers[workerID]; ok {
		w.Status = StatusOffline
	}
}

// StaleWorkers returns IDs of workers whose heartbeat is older than the configured timeout.
func (r *MemoryRegistry) StaleWorkers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cutoff := time.Now().Add(-r.timeout)
	var ids []string
	for _, w := range r.workers {
		if w.Status != StatusOffline && w.LastHeartbeat.Before(cutoff) {
			ids = append(ids, w.ID)
		}
	}
	return ids
}
