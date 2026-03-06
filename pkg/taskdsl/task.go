package taskdsl

import "time"

// Task represents a user-submitted execution request.
type Task struct {
	ID        string
	Prompt    string
	State     string
	Plan      *Plan
	CreatedAt time.Time
	UpdatedAt time.Time
}
