package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/persistence"
)

type stubAuditStore struct {
	byTask   map[string][]persistence.AuditRecord
	byAction map[string]persistence.AuditRecord
	global   []persistence.AuditRecord
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

func (s *stubAuditStore) Query(_ context.Context, query persistence.AuditQuery) ([]persistence.AuditRecord, error) {
	out := make([]persistence.AuditRecord, 0, len(s.global))
	for _, record := range s.global {
		if query.TaskID != "" && record.TaskID != query.TaskID {
			continue
		}
		if query.ActionID != "" && record.ActionID != query.ActionID {
			continue
		}
		if query.TenantID != "" && record.TenantID != query.TenantID {
			continue
		}
		if query.AgentName != "" && record.AgentName != query.AgentName {
			continue
		}
		if query.WorkerID != "" && record.WorkerID != query.WorkerID {
			continue
		}
		if query.FailedOnly && record.Error == "" && record.ExitCode == 0 {
			continue
		}
		out = append(out, record)
	}
	if query.Limit > 0 && len(out) > query.Limit {
		out = out[:query.Limit]
	}
	return out, nil
}

type stubReplayAPI struct {
	stubTaskAPI
	replay *access.TaskReplay
}

func (s stubReplayAPI) GetTaskReplay(_ context.Context, taskID string) (*access.TaskReplay, error) {
	if s.replay != nil {
		return s.replay, nil
	}
	return &access.TaskReplay{TaskID: taskID, State: "running"}, nil
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

func TestServer_TaskReplay_ReturnsProjectedReplay(t *testing.T) {
	replay := &access.TaskReplay{
		TaskID:    "task-123",
		State:     "succeeded",
		TenantID:  "tenant-1",
		AgentName: "ops-agent",
		Prompt:    "fix deployment",
		Summary: access.TaskReplaySummary{
			ActionCount:    2,
			CompletedCount: 2,
			FailedCount:    1,
		},
		Actions: []access.TaskReplayAction{
			{ActionID: "act-1", Status: "completed", Command: "echo hello", Stdout: "hello"},
			{ActionID: "act-2", Status: "failed", Command: "false", Error: "command failed"},
		},
	}
	srv := &Server{API: stubReplayAPI{replay: replay}}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/replay", nil)
	rec := httptest.NewRecorder()
	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp access.TaskReplay
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.TaskID != "task-123" {
		t.Fatalf("expected task-123, got %q", resp.TaskID)
	}
	if resp.Summary.FailedCount != 1 {
		t.Fatalf("expected 1 failed action, got %d", resp.Summary.FailedCount)
	}
	if len(resp.Actions) != 2 {
		t.Fatalf("expected 2 replay actions, got %d", len(resp.Actions))
	}
	if resp.Actions[1].Status != "failed" {
		t.Fatalf("expected failed action, got %+v", resp.Actions[1])
	}
}

func TestServer_GlobalAudit_ReturnsRecords(t *testing.T) {
	occurred := time.Unix(1_700_000_100, 0).UTC()
	audit := &stubAuditStore{global: []persistence.AuditRecord{
		{TaskID: "task-2", ActionID: "act-1", TenantID: "tenant-a", AgentName: "ops", ExitCode: 1, Error: "failed", OccurredAt: occurred},
		{TaskID: "task-1", ActionID: "act-1", TenantID: "tenant-a", AgentName: "ops", ExitCode: 0, OccurredAt: occurred.Add(-time.Minute)},
	}}
	srv := &Server{Audit: audit}

	req := httptest.NewRequest(http.MethodGet, "/v1/audit?tenant_id=tenant-a&failed=true&limit=1", nil)
	rec := httptest.NewRecorder()
	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp struct {
		Records []persistence.AuditRecord `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(resp.Records))
	}
	if resp.Records[0].TaskID != "task-2" {
		t.Fatalf("expected task-2 first, got %q", resp.Records[0].TaskID)
	}
}
