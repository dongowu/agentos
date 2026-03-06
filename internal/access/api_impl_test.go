package access

import (
	"context"
	"testing"

	"github.com/dongowu/agentos/internal/orchestration"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

type startTaskInputEngine struct {
	lastInput orchestration.StartTaskInput
}

func (e *startTaskInputEngine) StartTask(_ context.Context, _ string) (*taskdsl.Task, error) {
	return &taskdsl.Task{ID: "legacy", State: "queued"}, nil
}

func (e *startTaskInputEngine) StartTaskWithInput(_ context.Context, input orchestration.StartTaskInput) (*taskdsl.Task, error) {
	e.lastInput = input
	return &taskdsl.Task{ID: "task-123", State: "running"}, nil
}

func (e *startTaskInputEngine) Transition(_ context.Context, _ string, _ orchestration.TaskState) error {
	return nil
}

func (e *startTaskInputEngine) GetTask(_ context.Context, taskID string) (*taskdsl.Task, error) {
	return &taskdsl.Task{ID: taskID, State: "running"}, nil
}

func TestTaskSubmissionAPIImpl_CreateTask_PassesAgentContext(t *testing.T) {
	engine := &startTaskInputEngine{}
	api := NewTaskSubmissionAPIImpl(engine)

	_, err := api.CreateTask(context.Background(), CreateTaskRequest{
		Prompt:    "echo hello",
		TenantID:  "tenant-1",
		AgentName: "agent-1",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if engine.lastInput.AgentName != "agent-1" {
		t.Fatalf("expected agent-1, got %q", engine.lastInput.AgentName)
	}
	if engine.lastInput.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1, got %q", engine.lastInput.TenantID)
	}
}
