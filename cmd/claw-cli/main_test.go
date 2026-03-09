package main

import (
	"bytes"
	"encoding/json"
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
	if !strings.Contains(body, "agents: coder, demo") {
		t.Fatalf("expected agent list, got %q", body)
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
}
