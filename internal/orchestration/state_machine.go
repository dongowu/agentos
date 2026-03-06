package orchestration

import "fmt"

// TaskState represents the lifecycle state of a task.
type TaskState string

const (
	Pending    TaskState = "pending"
	Planning   TaskState = "planning"
	Queued     TaskState = "queued"
	Running    TaskState = "running"
	Evaluating TaskState = "evaluating"
	Succeeded  TaskState = "succeeded"
	Failed     TaskState = "failed"
)

// TaskStateMachine enforces valid state transitions.
type TaskStateMachine struct {
	allowed map[TaskState][]TaskState
}

// NewTaskStateMachine returns a state machine with the canonical transition table.
func NewTaskStateMachine() *TaskStateMachine {
	return &TaskStateMachine{
		allowed: map[TaskState][]TaskState{
			Pending:    {Planning},
			Planning:   {Queued, Failed},
			Queued:     {Running, Failed},
			Running:    {Evaluating, Failed},
			Evaluating: {Queued, Succeeded, Failed},
			Succeeded:  {},
			Failed:     {},
		},
	}
}

// Transition validates and returns the new state.
func (sm *TaskStateMachine) Transition(from, to TaskState) (TaskState, error) {
	allowed, ok := sm.allowed[from]
	if !ok {
		return from, fmt.Errorf("unknown state: %s", from)
	}
	for _, a := range allowed {
		if a == to {
			return to, nil
		}
	}
	return from, fmt.Errorf("invalid transition: %s -> %s", from, to)
}
