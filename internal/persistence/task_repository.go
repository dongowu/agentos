package persistence

import (
	"context"

	"github.com/dongowu/ai-orchestrator/pkg/taskdsl"
)

// TaskRepository persists tasks, plans, and execution history.
// Can be swapped for PostgreSQL without changing orchestration code.
type TaskRepository interface {
	Create(ctx context.Context, task *taskdsl.Task) error
	Get(ctx context.Context, id string) (*taskdsl.Task, error)
	Update(ctx context.Context, task *taskdsl.Task) error
}
