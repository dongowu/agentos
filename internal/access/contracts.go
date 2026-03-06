package access

import "context"

// CreateTaskRequest is the canonical task submission payload shared by HTTP and CLI.
type CreateTaskRequest struct {
	Prompt   string
	TenantID string
}

// CreateTaskResponse is returned after a task is accepted by the control plane.
type CreateTaskResponse struct {
	TaskID string
	State  string
}

// TaskSubmissionAPI accepts task requests from transport adapters.
type TaskSubmissionAPI interface {
	CreateTask(ctx context.Context, req CreateTaskRequest) (*CreateTaskResponse, error)
	GetTask(ctx context.Context, taskID string) (*CreateTaskResponse, error)
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
