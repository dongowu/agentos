package worker

import (
	"context"
	"time"
)

// WorkerStatus represents the current state of a worker.
type WorkerStatus string

const (
	StatusOnline  WorkerStatus = "online"
	StatusBusy    WorkerStatus = "busy"
	StatusOffline WorkerStatus = "offline"
)

// WorkerInfo describes a registered worker node.
type WorkerInfo struct {
	ID            string
	Addr          string       // gRPC address
	Capabilities  []string     // e.g. "native", "docker"
	Status        WorkerStatus
	LastHeartbeat time.Time
	ActiveTasks   int
	MaxTasks      int // concurrency limit
}

// Registry tracks worker nodes and their health.
type Registry interface {
	Register(ctx context.Context, info WorkerInfo) error
	Deregister(ctx context.Context, workerID string) error
	Heartbeat(ctx context.Context, workerID string, activeTasks int) error
	List(ctx context.Context) ([]WorkerInfo, error)
	GetAvailable(ctx context.Context) ([]WorkerInfo, error) // online + not full
}
