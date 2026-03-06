package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dongowu/agentos/internal/access"
)

type stubTaskAPI struct{}

func (stubTaskAPI) CreateTask(_ context.Context, req access.CreateTaskRequest) (*access.CreateTaskResponse, error) {
	return &access.CreateTaskResponse{TaskID: "task-123", State: "queued"}, nil
}

func (stubTaskAPI) GetTask(_ context.Context, taskID string) (*access.CreateTaskResponse, error) {
	return &access.CreateTaskResponse{TaskID: taskID, State: "running"}, nil
}

func TestServeAgentRun_ReturnsJSONTaskResponse(t *testing.T) {
	h := NewHandler(stubTaskAPI{})
	req := httptest.NewRequest(http.MethodPost, "/agent/run", strings.NewReader(`{"agent":"demo","task":"echo hello"}`))
	rec := httptest.NewRecorder()

	h.ServeAgentRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp AgentRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.TaskID != "task-123" {
		t.Fatalf("expected task_id task-123, got %q", resp.TaskID)
	}
	if resp.State != "queued" {
		t.Fatalf("expected state queued, got %q", resp.State)
	}
}

func TestServeAgentStatus_ReturnsJSONTaskResponse(t *testing.T) {
	h := NewHandler(stubTaskAPI{})
	req := httptest.NewRequest(http.MethodGet, "/agent/status?task_id=task-999", nil)
	rec := httptest.NewRecorder()

	h.ServeAgentStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp AgentRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.TaskID != "task-999" {
		t.Fatalf("expected task_id task-999, got %q", resp.TaskID)
	}
	if resp.State != "running" {
		t.Fatalf("expected state running, got %q", resp.State)
	}
}