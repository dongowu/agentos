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

type stubTaskAPI struct {
	lastReq access.CreateTaskRequest
}

type stubAgentManager struct {
	agents  map[string]bool
	runtime interface{ CheckPolicy(string) error }
}

func (m stubAgentManager) Get(name string) interface{ CheckPolicy(string) error } {
	if m.runtime != nil && m.agents[name] {
		return m.runtime
	}
	if m.agents[name] {
		return stubAgentRuntime{}
	}
	return nil
}

func (m stubAgentManager) List() []string {
	names := make([]string, 0, len(m.agents))
	for name := range m.agents {
		names = append(names, name)
	}
	return names
}

type stubAgentRuntime struct{}

func (stubAgentRuntime) CheckPolicy(string) error { return nil }

func (s *stubTaskAPI) CreateTask(_ context.Context, req access.CreateTaskRequest) (*access.CreateTaskResponse, error) {
	s.lastReq = req
	return &access.CreateTaskResponse{TaskID: "task-123", State: "queued"}, nil
}

func (s *stubTaskAPI) GetTask(_ context.Context, taskID string) (*access.CreateTaskResponse, error) {
	return &access.CreateTaskResponse{TaskID: taskID, State: "running"}, nil
}

func TestServeAgentRun_ReturnsJSONTaskResponse(t *testing.T) {
	api := &stubTaskAPI{}
	h := NewHandler(api)
	h.AgentManager = stubAgentManager{agents: map[string]bool{"demo": true}}
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
	if api.lastReq.AgentName != "demo" {
		t.Fatalf("expected agent name forwarded, got %q", api.lastReq.AgentName)
	}
	if api.lastReq.Prompt != "echo hello" {
		t.Fatalf("expected task forwarded as prompt, got %q", api.lastReq.Prompt)
	}
}

func TestServeAgentStatus_ReturnsJSONTaskResponse(t *testing.T) {
	api := &stubTaskAPI{}
	h := NewHandler(api)
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

func TestServeAgentRun_UnknownAgent_ReturnsNotFound(t *testing.T) {
	api := &stubTaskAPI{}
	h := NewHandler(api)
	h.AgentManager = stubAgentManager{agents: map[string]bool{"demo": true}}
	req := httptest.NewRequest(http.MethodPost, "/agent/run", strings.NewReader(`{"agent":"missing","task":"echo hello"}`))
	rec := httptest.NewRecorder()

	h.ServeAgentRun(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestServeAgentList_ReturnsLoadedAgents(t *testing.T) {
	api := &stubTaskAPI{}
	h := NewHandler(api)
	h.AgentManager = stubAgentManager{agents: map[string]bool{"demo": true, "coder": true}}
	req := httptest.NewRequest(http.MethodGet, "/agent/list", nil)
	rec := httptest.NewRecorder()

	h.ServeAgentList(rec, req)

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


type stubPromptAgentRuntime struct {
	prompt string
}

func (s stubPromptAgentRuntime) CheckPolicy(string) error { return nil }
func (s stubPromptAgentRuntime) BuildPrompt(task string) string {
	if s.prompt != "" {
		return s.prompt
	}
	return "agent-aware::" + task
}

func TestServeAgentRun_UsesAgentAwarePromptWhenAvailable(t *testing.T) {
	api := &stubTaskAPI{}
	h := NewHandler(api)
	h.AgentManager = stubAgentManager{
		agents:  map[string]bool{"demo": true},
		runtime: stubPromptAgentRuntime{prompt: "agent-aware::echo hello"},
	}
	req := httptest.NewRequest(http.MethodPost, "/agent/run", strings.NewReader(`{"agent":"demo","task":"echo hello"}`))
	rec := httptest.NewRecorder()

	h.ServeAgentRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if api.lastReq.AgentName != "demo" {
		t.Fatalf("expected agent name forwarded, got %q", api.lastReq.AgentName)
	}
	if api.lastReq.Prompt != "agent-aware::echo hello" {
		t.Fatalf("expected agent-aware prompt, got %q", api.lastReq.Prompt)
	}
}
