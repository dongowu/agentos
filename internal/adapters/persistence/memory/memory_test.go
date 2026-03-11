package memory

import (
	"context"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

func TestTaskRepository_RoundTripTenantID(t *testing.T) {
	repo := NewTaskRepository()
	task := &taskdsl.Task{
		ID:        "task-tenant",
		Prompt:    "echo hello",
		TenantID:  "tenant-a",
		State:     "queued",
		CreatedAt: time.Unix(1_700_000_000, 0).UTC(),
		UpdatedAt: time.Unix(1_700_000_001, 0).UTC(),
	}
	if err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected task, got nil")
	}
	if got.TenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %q", got.TenantID)
	}
}

func TestTaskRepository_ListRecoverable(t *testing.T) {
	repo := NewTaskRepository()
	tasks := []*taskdsl.Task{
		{ID: "task-succeeded", Prompt: "done", State: "succeeded", CreatedAt: time.Unix(1_700_000_000, 0).UTC(), UpdatedAt: time.Unix(1_700_000_000, 0).UTC()},
		{ID: "task-running", Prompt: "run", State: "running", CreatedAt: time.Unix(1_700_000_010, 0).UTC(), UpdatedAt: time.Unix(1_700_000_010, 0).UTC()},
		{ID: "task-queued", Prompt: "queue", State: "queued", CreatedAt: time.Unix(1_700_000_020, 0).UTC(), UpdatedAt: time.Unix(1_700_000_020, 0).UTC()},
	}
	for _, task := range tasks {
		if err := repo.Create(context.Background(), task); err != nil {
			t.Fatalf("Create(%s): %v", task.ID, err)
		}
	}

	got, err := repo.ListRecoverable(context.Background())
	if err != nil {
		t.Fatalf("ListRecoverable: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 recoverable tasks, got %d", len(got))
	}
	if got[0].ID != "task-running" || got[1].ID != "task-queued" {
		t.Fatalf("unexpected recoverable order: %q, %q", got[0].ID, got[1].ID)
	}
}

func TestAuditLogStore_QueryFiltersTenantFailureAndLimit(t *testing.T) {
	store := NewAuditLogStore()
	records := []persistence.AuditRecord{
		{TaskID: "task-1", ActionID: "act-1", TenantID: "tenant-a", AgentName: "ops", ExitCode: 0, OccurredAt: time.Unix(1_700_000_000, 0).UTC()},
		{TaskID: "task-2", ActionID: "act-1", TenantID: "tenant-a", AgentName: "ops", ExitCode: 1, Error: "failed", OccurredAt: time.Unix(1_700_000_010, 0).UTC()},
		{TaskID: "task-3", ActionID: "act-1", TenantID: "tenant-b", AgentName: "ops", ExitCode: 1, Error: "failed", OccurredAt: time.Unix(1_700_000_020, 0).UTC()},
	}
	for _, record := range records {
		if err := store.Append(context.Background(), record); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	got, err := store.Query(context.Background(), persistence.AuditQuery{TenantID: "tenant-a", FailedOnly: true, Limit: 1})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}
	if got[0].TaskID != "task-2" {
		t.Fatalf("expected newest tenant-a failed record task-2, got %q", got[0].TaskID)
	}
	if got[0].TenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %q", got[0].TenantID)
	}
}
