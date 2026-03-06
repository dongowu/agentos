package scheduler

import (
	"context"

	"github.com/dongowu/agentos/pkg/taskdsl"
)

// ActionResult is the outcome of a dispatched action.
type ActionResult struct {
	TaskID   string
	ActionID string
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	WorkerID string
	Error    error
}

// Scheduler distributes actions across available workers.
type Scheduler interface {
	// Submit queues an action for execution by any available worker.
	Submit(ctx context.Context, taskID string, action *taskdsl.Action) error
	// Results returns a channel that receives completed action results.
	Results() <-chan ActionResult
	// Close releases resources held by the scheduler.
	Close() error
}
