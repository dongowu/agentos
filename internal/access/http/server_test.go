package http

import (
	"bytes"
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

func TestServer_Handler_RequiresBearerTokenForGatewayRoutesWhenAuthConfigured(t *testing.T) {
	gw := gateway.NewHandler(stubTaskAPI{})
	gw.AgentManager = stubAgentManager{}
	srv := &Server{Gateway: gw, API: stubTaskAPI{}, Auth: &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}}

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{name: "agent list", method: http.MethodGet, path: "/agent/list"},
		{name: "agent status", method: http.MethodGet, path: "/agent/status?task_id=task-123"},
		{name: "agent run", method: http.MethodPost, path: "/agent/run", body: []byte(`{"agent":"demo","task":"echo hi"}`)},
		{name: "tool run", method: http.MethodPost, path: "/tool/run", body: []byte(`{"tool":"file.read","input":{"path":"/tmp/x"}}`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
			if len(tc.body) > 0 {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			srv.handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d", rec.Code)
			}
		})
	}
}
