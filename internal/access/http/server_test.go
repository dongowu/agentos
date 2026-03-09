package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/agent"
	"github.com/dongowu/agentos/internal/gateway"
	"github.com/dongowu/agentos/internal/worker"
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

func TestServer_ListAgents_ReturnsLoadedAgents(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "coder.yaml"), []byte("name: coder\ndescription: Writes and patches code\nmodel: gpt-4.1\ntools:\n  - bash\n  - file.read\nworkflow:\n  - plan\n  - execute\n"), 0644); err != nil {
		t.Fatalf("write coder agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ops.yaml"), []byte("name: ops\ndescription: Handles operator tasks\nmodel: gpt-4o-mini\ntools:\n  - kubectl\n"), 0644); err != nil {
		t.Fatalf("write ops agent: %v", err)
	}
	mgr := agent.NewManager()
	if err := mgr.LoadFromDir(dir); err != nil {
		t.Fatalf("load agents: %v", err)
	}
	srv := &Server{Agents: mgr}

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Agents []struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Model       string   `json:"model"`
			Tools       []string `json:"tools"`
			Workflow    []string `json:"workflow"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Agents))
	}
	if resp.Agents[0].Name != "coder" {
		t.Fatalf("expected first agent coder, got %q", resp.Agents[0].Name)
	}
	if resp.Agents[0].Description != "Writes and patches code" {
		t.Fatalf("expected coder description, got %q", resp.Agents[0].Description)
	}
	if resp.Agents[0].Model != "gpt-4.1" {
		t.Fatalf("expected coder model gpt-4.1, got %q", resp.Agents[0].Model)
	}
	if len(resp.Agents[0].Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(resp.Agents[0].Tools))
	}
	if len(resp.Agents[0].Workflow) != 2 {
		t.Fatalf("expected 2 workflow steps, got %d", len(resp.Agents[0].Workflow))
	}
	if resp.Agents[1].Name != "ops" {
		t.Fatalf("expected second agent ops, got %q", resp.Agents[1].Name)
	}
}

func TestServer_ListWorkers_ReturnsRegisteredWorkers(t *testing.T) {
	heartbeat := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	reg := &stubWorkerRegistry{workers: []worker.WorkerInfo{{
		ID:            "worker-1",
		Addr:          "127.0.0.1:9001",
		Capabilities:  []string{"docker", "native"},
		Status:        worker.StatusBusy,
		LastHeartbeat: heartbeat,
		ActiveTasks:   2,
		MaxTasks:      4,
	}}}
	srv := &Server{Workers: reg}

	req := httptest.NewRequest(http.MethodGet, "/v1/workers", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Workers []struct {
			ID            string   `json:"id"`
			Addr          string   `json:"addr"`
			Capabilities  []string `json:"capabilities"`
			Status        string   `json:"status"`
			LastHeartbeat string   `json:"last_heartbeat"`
			ActiveTasks   int      `json:"active_tasks"`
			MaxTasks      int      `json:"max_tasks"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(resp.Workers))
	}
	if resp.Workers[0].ID != "worker-1" {
		t.Fatalf("expected worker-1, got %q", resp.Workers[0].ID)
	}
	if resp.Workers[0].Status != string(worker.StatusBusy) {
		t.Fatalf("expected busy status, got %q", resp.Workers[0].Status)
	}
	if resp.Workers[0].LastHeartbeat != heartbeat.Format(time.RFC3339) {
		t.Fatalf("expected RFC3339 heartbeat, got %q", resp.Workers[0].LastHeartbeat)
	}
	if resp.Workers[0].ActiveTasks != 2 || resp.Workers[0].MaxTasks != 4 {
		t.Fatalf("expected task counts 2/4, got %d/%d", resp.Workers[0].ActiveTasks, resp.Workers[0].MaxTasks)
	}
}

type stubWorkerRegistry struct {
	workers []worker.WorkerInfo
}

func (*stubWorkerRegistry) Register(context.Context, worker.WorkerInfo) error { return nil }

func (*stubWorkerRegistry) Deregister(context.Context, string) error { return nil }

func (*stubWorkerRegistry) Heartbeat(context.Context, string, int) error { return nil }

func (r *stubWorkerRegistry) List(context.Context) ([]worker.WorkerInfo, error) {
	return append([]worker.WorkerInfo(nil), r.workers...), nil
}

func (*stubWorkerRegistry) GetAvailable(context.Context) ([]worker.WorkerInfo, error) {
	return nil, nil
}
