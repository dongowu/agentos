package access

import (
	"context"
	"time"
)

// CreateTaskRequest is the canonical task submission payload shared by HTTP and CLI.
type CreateTaskRequest struct {
	Prompt    string `json:"prompt"`
	TenantID  string `json:"tenant_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
}

// CreateTaskResponse is returned after a task is accepted by the control plane.
type CreateTaskResponse struct {
	TaskID string `json:"task_id"`
	State  string `json:"state"`
}

// TaskDetail contains control-plane metadata needed for authorization and read-path decisions.
type TaskDetail struct {
	TaskID   string `json:"task_id"`
	State    string `json:"state"`
	TenantID string `json:"tenant_id,omitempty"`
}

// TaskReplaySummary aggregates operator-facing replay counters.
type TaskReplaySummary struct {
	ActionCount    int `json:"action_count"`
	CompletedCount int `json:"completed_count"`
	FailedCount    int `json:"failed_count"`
}

// TaskReplayAction represents one planned or executed action in a replay projection.
type TaskReplayAction struct {
	ActionID    string     `json:"action_id"`
	Kind        string     `json:"kind,omitempty"`
	RuntimeEnv  string     `json:"runtime_env,omitempty"`
	Status      string     `json:"status"`
	Command     string     `json:"command,omitempty"`
	WorkerID    string     `json:"worker_id,omitempty"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	Stdout      string     `json:"stdout,omitempty"`
	Stderr      string     `json:"stderr,omitempty"`
	Error       string     `json:"error,omitempty"`
	SideEffects []string   `json:"side_effects,omitempty"`
	OccurredAt  *time.Time `json:"occurred_at,omitempty"`
}

// TaskReplay is a task-centric audit/replay projection.
type TaskReplay struct {
	TaskID    string             `json:"task_id"`
	State     string             `json:"state"`
	TenantID  string             `json:"tenant_id,omitempty"`
	AgentName string             `json:"agent_name,omitempty"`
	Prompt    string             `json:"prompt,omitempty"`
	Summary   TaskReplaySummary  `json:"summary"`
	Actions   []TaskReplayAction `json:"actions"`
}

// TaskSubmissionAPI accepts task requests from transport adapters.
type TaskSubmissionAPI interface {
	CreateTask(ctx context.Context, req CreateTaskRequest) (*CreateTaskResponse, error)
	GetTask(ctx context.Context, taskID string) (*CreateTaskResponse, error)
}

// TaskDetailAPI is an optional extension that exposes tenant metadata for read-path authorization.
type TaskDetailAPI interface {
	GetTaskDetail(ctx context.Context, taskID string) (*TaskDetail, error)
}

// TaskReplayAPI is an optional extension that exposes task replay projections.
type TaskReplayAPI interface {
	GetTaskReplay(ctx context.Context, taskID string) (*TaskReplay, error)
}

// AuthProvider resolves caller identity and tenant context.
type AuthProvider interface {
	Authenticate(ctx context.Context, token string) (*Principal, error)
}

// Principal represents the authenticated caller.
type Principal struct {
	Subject  string
	TenantID string
	Roles    []string
}
