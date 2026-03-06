package access

import (
	"context"
	"fmt"

	"github.com/agentos/agentos/internal/orchestration"
)

// TaskSubmissionAPIImpl implements TaskSubmissionAPI using orchestration.TaskEngine.
type TaskSubmissionAPIImpl struct {
	engine orchestration.TaskEngine
}

// NewTaskSubmissionAPIImpl returns an API backed by the given engine.
func NewTaskSubmissionAPIImpl(engine orchestration.TaskEngine) *TaskSubmissionAPIImpl {
	return &TaskSubmissionAPIImpl{engine: engine}
}

// CreateTask implements TaskSubmissionAPI.
func (a *TaskSubmissionAPIImpl) CreateTask(ctx context.Context, req CreateTaskRequest) (*CreateTaskResponse, error) {
	task, err := a.engine.StartTask(ctx, req.Prompt)
	if err != nil {
		return nil, fmt.Errorf("start task: %w", err)
	}
	return &CreateTaskResponse{TaskID: task.ID, State: task.State}, nil
}

// GetTask implements TaskSubmissionAPI.
func (a *TaskSubmissionAPIImpl) GetTask(ctx context.Context, taskID string) (*CreateTaskResponse, error) {
	task, err := a.engine.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return &CreateTaskResponse{TaskID: task.ID, State: task.State}, nil
}
