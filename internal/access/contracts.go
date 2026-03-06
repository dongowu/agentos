package access

import "context"

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
