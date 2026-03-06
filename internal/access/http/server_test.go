package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/gateway"
)

type stubTaskAPI struct{}

func (stubTaskAPI) CreateTask(_ context.Context, req access.CreateTaskRequest) (*access.CreateTaskResponse, error) {
	return &access.CreateTaskResponse{TaskID: "task-123", State: "queued"}, nil
}

func (stubTaskAPI) GetTask(_ context.Context, taskID string) (*access.CreateTaskResponse, error) {
	return &access.CreateTaskResponse{TaskID: taskID, State: "running"}, nil
}

type stubAgentManager struct{}

func (stubAgentManager) Get(name string) interface{ CheckPolicy(string) error } { return nil }
func (stubAgentManager) List() []string                                         { return []string{"demo", "coder"} }

func TestServer_Handler_RegistersAgentListRoute(t *testing.T) {
	gw := gateway.NewHandler(stubTaskAPI{})
	gw.AgentManager = stubAgentManager{}
	srv := &Server{Gateway: gw, API: stubTaskAPI{}}

	req := httptest.NewRequest(http.MethodGet, "/agent/list", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Agents []string `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Agents))
	}
}

func TestServer_Handler_HealthRoute(t *testing.T) {
	srv := &Server{API: stubTaskAPI{}}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
}
