package scheduler

import (
	"context"
	"log"

	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

// WorkerPool is the subset of worker.Pool needed by the scheduler.
type WorkerPool interface {
	SelectWorker(ctx context.Context) (string, error)
	Execute(ctx context.Context, workerID, taskID string, action *taskdsl.Action) (*runtimeclient.ExecutionResult, error)
}

// LocalScheduler dispatches actions directly to workers via the pool.
// Intended for dev mode when NATS is not available.
type LocalScheduler struct {
	pool    WorkerPool
	results chan ActionResult
}

// NewLocalScheduler creates a scheduler that dispatches via the worker pool.
func NewLocalScheduler(pool WorkerPool) *LocalScheduler {
	return &LocalScheduler{
		pool:    pool,
		results: make(chan ActionResult, 64),
	}
}

// Submit selects the least-loaded worker and dispatches the action.
// Execution runs in a goroutine; the result is sent to Results().
func (s *LocalScheduler) Submit(ctx context.Context, taskID string, action *taskdsl.Action) error {
	workerID, err := s.pool.SelectWorker(ctx)
	if err != nil {
		return err
	}

	dispatchCtx := context.WithoutCancel(ctx)
	go s.dispatch(dispatchCtx, workerID, taskID, action)
	return nil
}

func (s *LocalScheduler) dispatch(ctx context.Context, workerID, taskID string, action *taskdsl.Action) {
	result, err := s.pool.Execute(ctx, workerID, taskID, action)
	if err != nil {
		log.Printf("scheduler dispatch failed task=%s action=%s worker=%s: %v", taskID, action.ID, workerID, err)
	}

	ar := ActionResult{
		TaskID:   taskID,
		ActionID: action.ID,
		WorkerID: workerID,
		Error:    err,
	}
	if result != nil {
		ar.ExitCode = result.ExitCode
		ar.Stdout = result.Stdout
		ar.Stderr = result.Stderr
	}
	s.results <- ar
}

// Results returns the channel receiving completed action results.
func (s *LocalScheduler) Results() <-chan ActionResult {
	return s.results
}

// Close releases resources.
func (s *LocalScheduler) Close() error {
	return nil
}

var _ Scheduler = (*LocalScheduler)(nil)
