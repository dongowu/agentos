package access

import (
	"context"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/orchestration"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

type startTaskInputEngine struct {
	lastInput orchestration.StartTaskInput
	task      *taskdsl.Task
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
	if e.task != nil {
		return e.task, nil
	}
	return &taskdsl.Task{ID: taskID, State: "running"}, nil
}

type stubAccessAuditStore struct {
	records map[string]persistence.AuditRecord
}

func (s *stubAccessAuditStore) Append(_ context.Context, record persistence.AuditRecord) error {
	if s.records == nil {
		s.records = map[string]persistence.AuditRecord{}
	}
	s.records[record.TaskID+"/"+record.ActionID] = record
	return nil
}

func (s *stubAccessAuditStore) Get(_ context.Context, taskID, actionID string) (*persistence.AuditRecord, error) {
	record, ok := s.records[taskID+"/"+actionID]
	if !ok {
		return nil, nil
	}
	copyRecord := record
	return &copyRecord, nil
}

func (s *stubAccessAuditStore) ListByTask(_ context.Context, taskID string) ([]persistence.AuditRecord, error) {
	out := make([]persistence.AuditRecord, 0, len(s.records))
	for key, record := range s.records {
		if len(key) >= len(taskID)+1 && key[:len(taskID)+1] == taskID+"/" {
			out = append(out, record)
		}
	}
	return out, nil
}

func (s *stubAccessAuditStore) Query(_ context.Context, query persistence.AuditQuery) ([]persistence.AuditRecord, error) {
	out := make([]persistence.AuditRecord, 0, len(s.records))
	for _, record := range s.records {
		if query.TaskID != "" && record.TaskID != query.TaskID {
			continue
		}
		out = append(out, record)
	}
	return out, nil
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

func TestTaskSubmissionAPIImpl_GetTaskReplay_JoinsPlanAndAudit(t *testing.T) {
	occurred := time.Unix(1_700_000_000, 0).UTC()
	engine := &startTaskInputEngine{task: &taskdsl.Task{
		ID:        "task-123",
		Prompt:    "fix deployment",
		TenantID:  "tenant-1",
		AgentName: "ops-agent",
		State:     "succeeded",
		Plan: &taskdsl.Plan{Actions: []taskdsl.Action{
			{ID: "act-1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo hello"}},
			{ID: "act-2", Kind: "command.exec", RuntimeEnv: "docker", Payload: map[string]any{"cmd": "false"}},
		}},
	}}
	audit := &stubAccessAuditStore{records: map[string]persistence.AuditRecord{
		"task-123/act-1": {
			TaskID:     "task-123",
			ActionID:   "act-1",
			Command:    "echo hello",
			RuntimeEnv: "default",
			WorkerID:   "worker-1",
			ExitCode:   0,
			Stdout:     "hello",
			OccurredAt: occurred,
		},
		"task-123/act-2": {
			TaskID:     "task-123",
			ActionID:   "act-2",
			Command:    "false",
			RuntimeEnv: "docker",
			WorkerID:   "worker-2",
			ExitCode:   1,
			Stderr:     "boom",
			Error:      "command failed",
			OccurredAt: occurred.Add(time.Minute),
		},
	}}
	api := NewTaskSubmissionAPIImpl(engine).WithAuditStore(audit)

	replay, err := api.GetTaskReplay(context.Background(), "task-123")
	if err != nil {
		t.Fatalf("GetTaskReplay: %v", err)
	}
	if replay.TaskID != "task-123" {
		t.Fatalf("expected task-123, got %q", replay.TaskID)
	}
	if replay.AgentName != "ops-agent" {
		t.Fatalf("expected ops-agent, got %q", replay.AgentName)
	}
	if replay.Summary.ActionCount != 2 {
		t.Fatalf("expected 2 actions, got %d", replay.Summary.ActionCount)
	}
	if replay.Summary.CompletedCount != 2 {
		t.Fatalf("expected 2 completed actions, got %d", replay.Summary.CompletedCount)
	}
	if replay.Summary.FailedCount != 1 {
		t.Fatalf("expected 1 failed action, got %d", replay.Summary.FailedCount)
	}
	if len(replay.Actions) != 2 {
		t.Fatalf("expected 2 replay actions, got %d", len(replay.Actions))
	}
	if replay.Actions[0].ActionID != "act-1" || replay.Actions[0].Status != "completed" {
		t.Fatalf("unexpected first action: %+v", replay.Actions[0])
	}
	if replay.Actions[1].ActionID != "act-2" || replay.Actions[1].Status != "failed" {
		t.Fatalf("unexpected second action: %+v", replay.Actions[1])
	}
	if replay.Actions[1].Error != "command failed" {
		t.Fatalf("expected command failed, got %q", replay.Actions[1].Error)
	}
}

func TestTaskSubmissionAPIImpl_GetTaskReplay_IncludesPlannedPendingActions(t *testing.T) {
	engine := &startTaskInputEngine{task: &taskdsl.Task{
		ID:        "task-456",
		Prompt:    "deploy service",
		TenantID:  "tenant-1",
		AgentName: "deployer",
		State:     "running",
		Plan: &taskdsl.Plan{Actions: []taskdsl.Action{
			{ID: "act-1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo one"}},
			{ID: "act-2", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo two"}},
		}},
	}}
	audit := &stubAccessAuditStore{records: map[string]persistence.AuditRecord{
		"task-456/act-1": {
			TaskID:     "task-456",
			ActionID:   "act-1",
			Command:    "echo one",
			RuntimeEnv: "default",
			ExitCode:   0,
			Stdout:     "one",
		},
	}}
	api := NewTaskSubmissionAPIImpl(engine).WithAuditStore(audit)

	replay, err := api.GetTaskReplay(context.Background(), "task-456")
	if err != nil {
		t.Fatalf("GetTaskReplay: %v", err)
	}
	if len(replay.Actions) != 2 {
		t.Fatalf("expected 2 replay actions, got %d", len(replay.Actions))
	}
	if replay.Actions[1].ActionID != "act-2" {
		t.Fatalf("expected second planned action act-2, got %q", replay.Actions[1].ActionID)
	}
	if replay.Actions[1].Status != "pending" {
		t.Fatalf("expected pending action, got %+v", replay.Actions[1])
	}
	if replay.Summary.CompletedCount != 1 {
		t.Fatalf("expected 1 completed action, got %d", replay.Summary.CompletedCount)
	}
}
