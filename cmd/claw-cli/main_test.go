package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevCmd_ChecksHealthAndListsAgents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/workers":
			if r.URL.RawQuery != "available_only=true" {
				t.Fatalf("unexpected workers query: %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]int{
					"total":             1,
					"online":            1,
					"busy":              0,
					"offline":           0,
					"available_workers": 1,
				},
				"workers": []map[string]any{
					{
						"id":           "worker-1",
						"addr":         "127.0.0.1:5001",
						"status":       "online",
						"active_tasks": 0,
						"max_tasks":    2,
						"capabilities": []string{"native"},
					},
				},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"coder", "demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}
	body := stdout.String()
	if !strings.Contains(body, "server: ok") {
		t.Fatalf("expected health summary, got %q", body)
	}
	if !strings.Contains(body, "readiness: ok") {
		t.Fatalf("expected readiness summary, got %q", body)
	}
	if !strings.Contains(body, "available workers: worker-1") {
		t.Fatalf("expected available worker output, got %q", body)
	}
	if !strings.Contains(body, "agents: coder, demo") {
		t.Fatalf("expected agent list, got %q", body)
	}
}

func TestDevCmd_ShowsAvailableCapabilityBreakdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/workers":
			if r.URL.RawQuery != "available_only=true" {
				t.Fatalf("unexpected workers query: %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{
					"total":             2,
					"online":            2,
					"busy":              0,
					"offline":           0,
					"available_workers": 2,
					"capabilities": []map[string]any{
						{"name": "docker", "total": 1, "online": 1, "busy": 0, "offline": 0, "available_workers": 1},
						{"name": "native", "total": 1, "online": 1, "busy": 0, "offline": 0, "available_workers": 1},
					},
				},
				"workers": []map[string]any{
					{"id": "worker-1"},
					{"id": "worker-2"},
				},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"coder", "demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}
	body := stdout.String()
	if !strings.Contains(body, "available by capability: docker=1, native=1") {
		t.Fatalf("expected available capability breakdown, got %q", body)
	}
}

func TestDevCmd_ShowsCapabilityWarningsFromReadyProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":            "ok",
				"capacity_warnings": []string{"no available workers for capability docker"},
			})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{
					"total":             1,
					"online":            1,
					"busy":              0,
					"offline":           0,
					"available_workers": 1,
					"capabilities": []map[string]any{
						{"name": "native", "total": 1, "online": 1, "busy": 0, "offline": 0, "available_workers": 1},
					},
				},
				"workers": []map[string]any{{"id": "worker-1"}},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}
	body := stdout.String()
	if !strings.Contains(body, "capacity warnings: no available workers for capability docker") {
		t.Fatalf("expected readiness capability warnings, got %q", body)
	}
}

func TestDevCmd_ShowsDegradedReadinessReasons(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":           "degraded",
				"degraded_reasons": []string{"no available workers"},
			})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]int{
					"total":             0,
					"online":            0,
					"busy":              0,
					"offline":           0,
					"available_workers": 0,
				},
				"workers": []map[string]any{},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}
	body := stdout.String()
	if !strings.Contains(body, "server: ok") {
		t.Fatalf("expected health summary, got %q", body)
	}
	if !strings.Contains(body, "readiness: degraded (no available workers)") {
		t.Fatalf("expected degraded readiness summary, got %q", body)
	}
	if !strings.Contains(body, "available workers: (none)") {
		t.Fatalf("expected empty worker summary, got %q", body)
	}
}

func TestDevCmd_OutputJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":            "ok",
				"capacity_warnings": []string{"no available workers for capability docker"},
			})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":            "ok",
				"capacity_warnings": []string{"no available workers for capability docker"},
			})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{
					"capabilities": []map[string]any{
						{"name": "native", "available_workers": 1},
					},
				},
				"workers": []map[string]any{{"id": "worker-1"}},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}

	var resp struct {
		Health  healthResponse     `json:"health"`
		Ready   healthResponse     `json:"ready"`
		Workers workerListResponse `json:"workers"`
		Agents  []string           `json:"agents"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", stdout.String(), err)
	}
	if resp.Health.Status != "ok" || resp.Ready.Status != "ok" {
		t.Fatalf("unexpected health payload: %+v", resp)
	}
	if len(resp.Workers.Workers) != 1 || resp.Workers.Workers[0].ID != "worker-1" {
		t.Fatalf("unexpected worker payload: %+v", resp.Workers)
	}
	if len(resp.Agents) != 1 || resp.Agents[0] != "demo" {
		t.Fatalf("unexpected agents payload: %+v", resp.Agents)
	}
}

func TestDevCmd_OutputJSON_IncludesSchemaVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{},
				"workers": []map[string]any{},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}

	var resp struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", stdout.String(), err)
	}
	if resp.SchemaVersion != "v1" {
		t.Fatalf("expected schema_version v1, got %+v", resp)
	}
}

func TestDevCmd_OutputJSON_WorkersSectionOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{},
				"workers": []map[string]any{{"id": "worker-1"}},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--output", "json", "--section", "workers"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", stdout.String(), err)
	}
	if _, ok := payload["schema_version"]; !ok {
		t.Fatalf("expected schema_version key, got %+v", payload)
	}
	if _, ok := payload["workers"]; !ok {
		t.Fatalf("expected workers key, got %+v", payload)
	}
	if _, ok := payload["health"]; ok {
		t.Fatalf("expected health key omitted, got %+v", payload)
	}
	if _, ok := payload["ready"]; ok {
		t.Fatalf("expected ready key omitted, got %+v", payload)
	}
	if _, ok := payload["agents"]; ok {
		t.Fatalf("expected agents key omitted, got %+v", payload)
	}
}

func TestDevCmd_OutputJSON_MultipleSections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{},
				"workers": []map[string]any{{"id": "worker-1"}},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--output", "json", "--section", "health,workers"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", stdout.String(), err)
	}
	if _, ok := payload["schema_version"]; !ok {
		t.Fatalf("expected schema_version key, got %+v", payload)
	}
	if _, ok := payload["health"]; !ok {
		t.Fatalf("expected health key, got %+v", payload)
	}
	if _, ok := payload["workers"]; !ok {
		t.Fatalf("expected workers key, got %+v", payload)
	}
	if _, ok := payload["ready"]; ok {
		t.Fatalf("expected ready key omitted, got %+v", payload)
	}
	if _, ok := payload["agents"]; ok {
		t.Fatalf("expected agents key omitted, got %+v", payload)
	}
}

func TestDevCmd_RequireReadyFailsWhenDegraded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":           "degraded",
				"degraded_reasons": []string{"no available workers"},
			})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{},
				"workers": []map[string]any{},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--require-ready"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "readiness requirement failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "readiness: degraded (no available workers)") {
		t.Fatalf("expected readiness diagnostics before failure, got %q", stdout.String())
	}
}

func TestDevCmd_RequireCapabilityFailsWhenUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{
					"capabilities": []map[string]any{
						{"name": "native", "available_workers": 1},
					},
				},
				"workers": []map[string]any{{"id": "worker-1"}},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--require-capability", "native,docker"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "required capabilities unavailable: docker") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "available by capability: native=1") {
		t.Fatalf("expected capability diagnostics before failure, got %q", stdout.String())
	}
}

func TestDevCmd_RejectsSectionWithoutJSONOutput(t *testing.T) {
	cmd := devCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--section", "workers"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "section requires --output json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDevCmd_RejectsUnknownJSONSection(t *testing.T) {
	cmd := devCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--output", "json", "--section", "tasks"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported section") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDevCmd_OutputTextFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{},
				"workers": []map[string]any{{"id": "worker-1"}},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--output", "text"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}
	if !strings.Contains(stdout.String(), "server: ok") {
		t.Fatalf("expected text diagnostics, got %q", stdout.String())
	}
}

func TestDevCmd_RejectsUnknownOutputMode(t *testing.T) {
	cmd := devCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--output", "yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDevCmd_LoadsAgentFileAndSubmitsTask(t *testing.T) {
	var captured struct {
		Agent string `json:"agent"`
		Task  string `json:"task"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agent/run" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(apiResponse{TaskID: "task-123", State: "queued"})
	}))
	defer server.Close()

	dir := t.TempDir()
	agentPath := filepath.Join(dir, "demo.yaml")
	if err := os.WriteFile(agentPath, []byte("name: demo\nmodel: gpt-4o\n"), 0o644); err != nil {
		t.Fatalf("write agent yaml: %v", err)
	}

	serverURL = server.URL
	cmd := devCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{agentPath, "echo hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}
	if captured.Agent != "demo" || captured.Task != "echo hello" {
		t.Fatalf("expected dev cmd to submit agent/task, got agent=%q task=%q", captured.Agent, captured.Task)
	}
	if !strings.Contains(stdout.String(), "task task-123 created") {
		t.Fatalf("expected task creation output, got %q", stdout.String())
	}
}

func TestRunCmd_SendsBearerTokenWhenConfigured(t *testing.T) {
	authToken = "token-123"
	defer func() { authToken = "" }()

	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/agent/run" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(apiResponse{TaskID: "task-1", State: "queued"})
	}))
	defer server.Close()

	dir := t.TempDir()
	agentPath := filepath.Join(dir, "demo.yaml")
	if err := os.WriteFile(agentPath, []byte("name: demo\nmodel: gpt-4o\n"), 0o644); err != nil {
		t.Fatalf("write agent yaml: %v", err)
	}

	serverURL = server.URL
	cmd := runCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{agentPath, "echo hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute run cmd: %v", err)
	}
	if auth != "Bearer token-123" {
		t.Fatalf("expected bearer auth, got %q", auth)
	}
}

func TestStatusCmd_SendsBearerTokenWhenConfigured(t *testing.T) {
	authToken = "token-xyz"
	defer func() { authToken = "" }()

	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(apiResponse{TaskID: "task-9", State: "running"})
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := statusCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"task-9"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute status cmd: %v", err)
	}
	if auth != "Bearer token-xyz" {
		t.Fatalf("expected bearer auth, got %q", auth)
	}
}

func TestDevCmd_DiagnosticsSendBearerTokenWhenConfigured(t *testing.T) {
	authToken = "token-dev"
	defer func() { authToken = "" }()

	seen := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path] = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]int{
					"total":             1,
					"online":            1,
					"busy":              0,
					"offline":           0,
					"available_workers": 1,
				},
				"workers": []map[string]any{{"id": "worker-1"}},
			})
		case "/agent/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []string{"demo"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL = server.URL
	cmd := devCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dev cmd: %v", err)
	}
	if seen["/agent/list"] != "Bearer token-dev" {
		t.Fatalf("expected bearer auth on /agent/list, got %q", seen["/agent/list"])
	}
	if seen["/ready"] != "Bearer token-dev" {
		t.Fatalf("expected bearer auth on /ready, got %q", seen["/ready"])
	}
	if seen["/v1/workers"] != "Bearer token-dev" {
		t.Fatalf("expected bearer auth on /v1/workers, got %q", seen["/v1/workers"])
	}
}
