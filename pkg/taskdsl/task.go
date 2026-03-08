package taskdsl

import "time"

// Task represents a user-submitted execution request.
type Task struct {
	ID        string
	Prompt    string
	State     string
	Plan      *Plan
	Result    string // final answer from agent loop
	CreatedAt time.Time
	UpdatedAt time.Time
}
