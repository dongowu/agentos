package worker

import (
	"context"
	"time"
)

// EventPublisher is the subset of messaging.EventBus needed by HealthMonitor.
// Keeps the worker package decoupled from the messaging package.
type EventPublisher interface {
	Publish(ctx context.Context, topic string, payload any) error
}

// HealthMonitor periodically checks worker heartbeats and marks stale workers offline.
type HealthMonitor struct {
	registry *MemoryRegistry
	bus      EventPublisher
	timeout  time.Duration
	interval time.Duration
}

// NewHealthMonitor creates a monitor that scans every interval and considers
// workers stale after timeout since their last heartbeat.
func NewHealthMonitor(registry *MemoryRegistry, bus EventPublisher, timeout, interval time.Duration) *HealthMonitor {
	return &HealthMonitor{
		registry: registry,
		bus:      bus,
		timeout:  timeout,
		interval: interval,
	}
}

// Start runs the health check loop until ctx is cancelled.
func (m *HealthMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.check(ctx)
		}
	}
}

func (m *HealthMonitor) check(ctx context.Context) {
	stale := m.registry.StaleWorkers()
	for _, id := range stale {
		m.registry.MarkOffline(id)
		_ = m.bus.Publish(ctx, "worker.offline", map[string]string{"worker_id": id})
	}
}
