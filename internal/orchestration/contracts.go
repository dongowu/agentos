package orchestration

import (
	"context"

	"github.com/dongowu/ai-orchestrator/pkg/taskdsl"
)

// PlanInput is the normalized planner input after auth, validation, and request shaping.
type PlanInput struct {
	TaskID   string
	Prompt   string
	TenantID string
}

// Planner converts a prompt into a structured plan.
type Planner interface {
	Plan(ctx context.Context, input PlanInput) (*taskdsl.Plan, error)
}

// TaskEngine owns task lifecycle transitions and retry decisions.
type TaskEngine interface {
	StartTask(ctx context.Context, prompt string) (*taskdsl.Task, error)
	Transition(ctx context.Context, taskID string, to TaskState) error
	GetTask(ctx context.Context, taskID string) (*taskdsl.Task, error)
}

// SkillResolver maps an action to the runtime profile and skill contract it needs.
type SkillResolver interface {
	Resolve(action *taskdsl.Action) (runtimeProfile string, err error)
}

// MemoryProvider is intentionally optional and keeps retrieval logic behind an interface.
type MemoryProvider interface {
	Recall(ctx context.Context, taskID string, query string) ([]MemoryEntry, error)
	Store(ctx context.Context, taskID string, entries []MemoryEntry) error
}

// MemoryEntry is a future-facing retrieval unit for long-term memory.
type MemoryEntry struct {
	Key      string
	Content  string
	Metadata map[string]string
}
