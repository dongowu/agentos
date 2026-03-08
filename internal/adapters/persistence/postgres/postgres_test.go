package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

func TestTaskRepository_RoundTripTenantID(t *testing.T) {
	dsn := os.Getenv("AGENTOS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("AGENTOS_TEST_POSTGRES_DSN not set")
	}
	repo, err := NewTaskRepository(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewTaskRepository: %v", err)
	}
	defer repo.Close()

	task := &taskdsl.Task{
		ID:        "task-tenant-postgres",
		Prompt:    "echo hello",
		TenantID:  "tenant-pg",
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
	if got.TenantID != "tenant-pg" {
		t.Fatalf("expected tenant-pg, got %q", got.TenantID)
	}
}

func TestAuditLogStore_QueryFiltersTenantFailureAndLimit(t *testing.T) {
	dsn := os.Getenv("AGENTOS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("AGENTOS_TEST_POSTGRES_DSN not set")
	}
	store, err := NewAuditLogStore(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewAuditLogStore: %v", err)
	}
	defer store.Close()

	records := []persistence.AuditRecord{
		{TaskID: "task-audit-pg-1", ActionID: "act-1", TenantID: "tenant-a", AgentName: "ops", ExitCode: 0, OccurredAt: time.Unix(1_700_000_100, 0).UTC()},
		{TaskID: "task-audit-pg-2", ActionID: "act-1", TenantID: "tenant-a", AgentName: "ops", ExitCode: 1, Error: "failed", OccurredAt: time.Unix(1_700_000_110, 0).UTC()},
		{TaskID: "task-audit-pg-3", ActionID: "act-1", TenantID: "tenant-b", AgentName: "ops", ExitCode: 1, Error: "failed", OccurredAt: time.Unix(1_700_000_120, 0).UTC()},
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
	if got[0].TaskID != "task-audit-pg-2" {
		t.Fatalf("expected tenant-a newest failed record, got %q", got[0].TaskID)
	}
	if got[0].TenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %q", got[0].TenantID)
	}
}
