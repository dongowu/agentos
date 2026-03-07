package persistence

import (
	"context"
	"time"
)

// AuditRecord captures side effects and raw commands for later review.
type AuditRecord struct {
	TaskID      string    `json:"task_id"`
	ActionID    string    `json:"action_id"`
	Command     string    `json:"command"`
	RuntimeEnv  string    `json:"runtime_env"`
	WorkerID    string    `json:"worker_id"`
	ExitCode    int       `json:"exit_code"`
	Stdout      string    `json:"stdout"`
	Stderr      string    `json:"stderr"`
	Error       string    `json:"error,omitempty"`
	SideEffects []string  `json:"side_effects,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// AuditLogStore persists execution audit records.
type AuditLogStore interface {
	Append(ctx context.Context, record AuditRecord) error
	Get(ctx context.Context, taskID, actionID string) (*AuditRecord, error)
	ListByTask(ctx context.Context, taskID string) ([]AuditRecord, error)
}
