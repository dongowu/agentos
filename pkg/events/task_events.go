package events

import "time"

// TaskCreated is emitted when a task is first created.
type TaskCreated struct {
	TaskID   string
	Prompt   string
	Occurred time.Time
}

// TaskPlanned is emitted when a plan is attached to a task.
type TaskPlanned struct {
	TaskID   string
	ActionCount int
	Occurred time.Time
}

// ActionDispatched is emitted when an action is sent to a worker.
type ActionDispatched struct {
	TaskID   string
	ActionID string
	Occurred time.Time
}

// ActionCompleted is emitted when a worker returns a result.
type ActionCompleted struct {
	TaskID   string
	ActionID string
	ExitCode int
	Occurred time.Time
}
