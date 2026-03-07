package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/persistence"
)

type stubAuditStore struct {
	byTask   map[string][]persistence.AuditRecord
	byAction map[string]persistence.AuditRecord
}

func (s *stubAuditStore) Append(_ context.Context, record persistence.AuditRecord) error {
	return nil
}

func (s *stubAuditStore) Get(_ context.Context, taskID, actionID string) (*persistence.AuditRecord, error) {
	record, ok := s.byAction[taskID+"/"+actionID]
	if !ok {
		return nil, nil
	}
	copyRecord := record
	return &copyRecord, nil
}

func (s *stubAuditStore) ListByTask(_ context.Context, taskID string) ([]persistence.AuditRecord, error) {
	return append([]persistence.AuditRecord(nil), s.byTask[taskID]...), nil
}

func TestServer_TaskAudit_ReturnsPersistedRecords(t *testing.T) {
	occurred := time.Unix(1_700_000_000, 0).UTC()
	audit := &stubAuditStore{byTask: map[string][]persistence.AuditRecord{
		"task-123": {
			{TaskID: "task-123", ActionID: "act-1", Command: "echo hello", ExitCode: 0, WorkerID: "worker-1", Stdout: "hello", OccurredAt: occurred},
		},
	}}
	srv := &Server{API: stubTaskAPI{}, Audit: audit}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/audit", nil)
	rec := httptest.NewRecorder()
	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp struct {
		TaskID  string                    `json:"task_id"`
		Records []persistence.AuditRecord `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.TaskID != "task-123" {
		t.Fatalf("expected task id task-123, got %q", resp.TaskID)
	}
	if len(resp.Records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(resp.Records))
	}
	if resp.Records[0].Command != "echo hello" {
		t.Fatalf("expected command echo hello, got %q", resp.Records[0].Command)
	}
}

func TestServer_ActionAudit_ReturnsSingleRecord(t *testing.T) {
	occurred := time.Unix(1_700_000_000, 0).UTC()
	audit := &stubAuditStore{byAction: map[string]persistence.AuditRecord{
		"task-123/act-1": {TaskID: "task-123", ActionID: "act-1", Command: "echo hello", ExitCode: 0, WorkerID: "worker-1", Stdout: "hello", OccurredAt: occurred},
	}}
	srv := &Server{API: stubTaskAPI{}, Audit: audit}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/actions/act-1/audit", nil)
	rec := httptest.NewRecorder()
	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var record persistence.AuditRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if record.ActionID != "act-1" {
		t.Fatalf("expected action id act-1, got %q", record.ActionID)
	}
	if record.WorkerID != "worker-1" {
		t.Fatalf("expected worker worker-1, got %q", record.WorkerID)
	}
}
