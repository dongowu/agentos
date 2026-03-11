package orchestration

import (
	"context"
	"strings"
	"testing"
	"time"

	persmemory "github.com/dongowu/agentos/internal/adapters/persistence/memory"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

func TestEngineImpl_RecoverTasks_RequeuesQueuedTaskFromNextPendingAction(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	sched := newMockScheduler()
	audit := &mockAuditStore{
		records: []persistence.AuditRecord{
			{TaskID: "task-recover-queued", ActionID: "action-1", ExitCode: 0},
		},
	}
	engine := NewEngineImpl(repo, nil, nil, nil, nil, nil, sched).WithAuditStore(audit)
	task := &taskdsl.Task{
		ID:    "task-recover-queued",
		State: string(Queued),
		Plan: &taskdsl.Plan{Actions: []taskdsl.Action{
			{ID: "action-1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo first"}},
			{ID: "action-2", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo second"}},
		}},
		CreatedAt: time.Unix(1_700_000_000, 0).UTC(),
		UpdatedAt: time.Unix(1_700_000_001, 0).UTC(),
	}
	if err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := engine.RecoverTasks(context.Background(), time.Hour); err != nil {
		t.Fatalf("RecoverTasks: %v", err)
	}

	if len(sched.submitted) != 1 {
		t.Fatalf("expected 1 resubmitted action, got %d", len(sched.submitted))
	}
	if sched.submitted[0].action.ID != "action-2" {
		t.Fatalf("expected action-2 to be recovered, got %s", sched.submitted[0].action.ID)
	}
	got, err := repo.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.State != string(Running) {
		t.Fatalf("expected recovered task to be running, got %s", got.State)
	}
}

func TestEngineImpl_RecoverTasks_FailsStaleRunningTask(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	sched := newMockScheduler()
	engine := NewEngineImpl(repo, nil, nil, nil, nil, nil, sched)
	task := &taskdsl.Task{
		ID:        "task-stale-running",
		State:     string(Running),
		CreatedAt: time.Unix(1_700_000_000, 0).UTC(),
		UpdatedAt: time.Now().Add(-2 * time.Hour),
	}
	if err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := engine.RecoverTasks(context.Background(), 30*time.Minute); err != nil {
		t.Fatalf("RecoverTasks: %v", err)
	}

	got, err := repo.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.State != string(Failed) {
		t.Fatalf("expected stale running task to fail, got %s", got.State)
	}
	if !strings.Contains(got.Result, "startup recovery") || !strings.Contains(got.Result, "stale") {
		t.Fatalf("expected explicit recovery reason, got %q", got.Result)
	}
	if len(sched.submitted) != 0 {
		t.Fatalf("expected no resubmission for stale running task, got %d", len(sched.submitted))
	}
}

func TestEngineImpl_RecoverTasks_LeavesTerminalTasksUntouched(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	sched := newMockScheduler()
	engine := NewEngineImpl(repo, nil, nil, nil, nil, nil, sched)
	tasks := []*taskdsl.Task{
		{ID: "task-succeeded", State: string(Succeeded), Result: "done", CreatedAt: time.Unix(1_700_000_100, 0).UTC(), UpdatedAt: time.Unix(1_700_000_100, 0).UTC()},
		{ID: "task-failed", State: string(Failed), Result: "boom", CreatedAt: time.Unix(1_700_000_200, 0).UTC(), UpdatedAt: time.Unix(1_700_000_200, 0).UTC()},
	}
	for _, task := range tasks {
		if err := repo.Create(context.Background(), task); err != nil {
			t.Fatalf("Create(%s): %v", task.ID, err)
		}
	}

	if err := engine.RecoverTasks(context.Background(), time.Minute); err != nil {
		t.Fatalf("RecoverTasks: %v", err)
	}

	for _, task := range tasks {
		got, err := repo.Get(context.Background(), task.ID)
		if err != nil {
			t.Fatalf("Get(%s): %v", task.ID, err)
		}
		if got.State != task.State || got.Result != task.Result {
			t.Fatalf("expected terminal task %s unchanged, got state=%s result=%q", task.ID, got.State, got.Result)
		}
	}
	if len(sched.submitted) != 0 {
		t.Fatalf("expected no recovered submissions, got %d", len(sched.submitted))
	}
}
