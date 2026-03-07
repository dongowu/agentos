package events

import "time"

// TaskCreated is emitted when a task is first created.
type TaskCreated struct {
	TaskID   string    `json:"task_id"`
	Prompt   string    `json:"prompt"`
	Occurred time.Time `json:"occurred"`
}

// TaskPlanned is emitted when a plan is attached to a task.
type TaskPlanned struct {
	TaskID      string    `json:"task_id"`
	ActionCount int       `json:"action_count"`
	Occurred    time.Time `json:"occurred"`
}

// ActionDispatched is emitted when an action is sent to a worker.
type ActionDispatched struct {
	TaskID   string    `json:"task_id"`
	ActionID string    `json:"action_id"`
	Occurred time.Time `json:"occurred"`
}

// ActionOutputChunk is emitted while an action is still running.
type ActionOutputChunk struct {
	TaskID   string    `json:"task_id"`
	ActionID string    `json:"action_id"`
	Kind     string    `json:"kind"`
	Data     []byte    `json:"data,omitempty"`
	Text     string    `json:"text,omitempty"`
	Occurred time.Time `json:"occurred"`
}

// ActionCompleted is emitted when a worker returns a result.
type ActionCompleted struct {
	TaskID   string    `json:"task_id"`
	ActionID string    `json:"action_id"`
	ExitCode int       `json:"exit_code"`
	Stdout   string    `json:"stdout,omitempty"`
	Stderr   string    `json:"stderr,omitempty"`
	WorkerID string    `json:"worker_id,omitempty"`
	Error    string    `json:"error,omitempty"`
	Occurred time.Time `json:"occurred"`
}
