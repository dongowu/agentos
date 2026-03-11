package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/dongowu/agentos/pkg/taskdsl"
)

// RecoverTasks repairs queued/running tasks left behind by a prior process.
func (e *EngineImpl) RecoverTasks(ctx context.Context, staleRunningTimeout time.Duration) error {
	if e.repo == nil {
		return nil
	}

	tasks, err := e.repo.ListRecoverable(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, task := range tasks {
		if task == nil {
			continue
		}
		switch TaskState(task.State) {
		case Queued:
			if err := e.recoverQueuedTask(ctx, task); err != nil {
				return err
			}
		case Running:
			if staleRunningTimeout > 0 && now.Sub(task.UpdatedAt) < staleRunningTimeout {
				continue
			}
			task.Result = fmt.Sprintf("startup recovery marked stale running task as failed (last update %s)", task.UpdatedAt.UTC().Format(time.RFC3339))
			if err := e.transition(ctx, task, Failed); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *EngineImpl) recoverQueuedTask(ctx context.Context, task *taskdsl.Task) error {
	if task == nil || task.Plan == nil || len(task.Plan.Actions) == 0 || e.scheduler == nil {
		return nil
	}

	nextIndex, err := e.nextRecoverableActionIndex(ctx, task)
	if err != nil {
		return err
	}
	if nextIndex < 0 {
		return nil
	}

	action := &task.Plan.Actions[nextIndex]
	if err := e.prepareAction(ctx, task, action); err != nil {
		return err
	}
	e.publishActionDispatched(ctx, task.ID, action.ID)
	if err := e.transition(ctx, task, Running); err != nil {
		return err
	}
	if err := e.submitScheduledAction(ctx, task.ID, action); err != nil {
		task.Result = fmt.Sprintf("startup recovery failed to resubmit action %s: %v", action.ID, err)
		if e.executor != nil || e.actionBridge != nil {
			_, directErr := e.executeDirectFromIndex(ctx, task, nextIndex, true, true)
			if directErr != nil {
				_ = e.transition(ctx, task, Failed)
				return fmt.Errorf("recover queued task: scheduler submit: %w; direct execute: %w", err, directErr)
			}
			return nil
		}
		_ = e.transition(ctx, task, Failed)
		return nil
	}
	return nil
}

func (e *EngineImpl) nextRecoverableActionIndex(ctx context.Context, task *taskdsl.Task) (int, error) {
	if task == nil || task.Plan == nil {
		return -1, nil
	}
	if e.auditStore == nil {
		if len(task.Plan.Actions) == 0 {
			return -1, nil
		}
		return 0, nil
	}

	records, err := e.auditStore.ListByTask(ctx, task.ID)
	if err != nil {
		return -1, err
	}
	completed := make(map[string]struct{}, len(records))
	for _, record := range records {
		if record.Error != "" || record.ExitCode != 0 {
			continue
		}
		completed[record.ActionID] = struct{}{}
	}
	for i := range task.Plan.Actions {
		if _, ok := completed[task.Plan.Actions[i].ID]; ok {
			continue
		}
		return i, nil
	}
	return -1, nil
}
