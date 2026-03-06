package persistence

import "context"

// AuditRecord captures side effects and raw commands for later review.
type AuditRecord struct {
	TaskID      string
	ActionID    string
	Command     string
	ExitCode    int
	SideEffects []string
}

// AuditLogStore persists execution audit records.
type AuditLogStore interface {
	Append(ctx context.Context, record AuditRecord) error
}
