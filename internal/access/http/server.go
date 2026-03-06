package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/gateway"
)

// Server exposes the HTTP API for task submission, agent run, and tool run.
type Server struct {
	Addr    string
	API     access.TaskSubmissionAPI
	Gateway *gateway.Handler
	srv     *http.Server
}

// Start begins listening.
func (s *Server) Start() error {
	s.srv = &http.Server{Addr: s.Addr, Handler: s.handler()}
	return s.srv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/tasks", s.handleTasks)
	mux.HandleFunc("/v1/tasks/", s.handleTaskByID)
	if s.Gateway != nil {
		mux.HandleFunc("/agent/run", s.Gateway.ServeAgentRun)
		mux.HandleFunc("/agent/status", s.Gateway.ServeAgentStatus)
		mux.HandleFunc("/agent/list", s.Gateway.ServeAgentList)
		mux.HandleFunc("/tool/run", s.Gateway.ServeToolRun)
	}
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/tasks" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.handleCreateTask(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
	if id == "" {
		http.Error(w, `{"error":"task id required"}`, http.StatusBadRequest)
		return
	}
	s.handleGetTask(w, r, id)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if s.API == nil {
		http.Error(w, `{"error":"api not configured"}`, http.StatusInternalServerError)
		return
	}
	var req struct {
		Prompt    string `json:"prompt"`
		TenantID  string `json:"tenant_id,omitempty"`
		AgentName string `json:"agent_name,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Prompt == "" {
		http.Error(w, `{"error":"prompt required"}`, http.StatusBadRequest)
		return
	}
	resp, err := s.API.CreateTask(r.Context(), access.CreateTaskRequest{Prompt: req.Prompt, TenantID: req.TenantID, AgentName: req.AgentName})
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request, taskID string) {
	if s.API == nil {
		http.Error(w, `{"error":"api not configured"}`, http.StatusInternalServerError)
		return
	}
	resp, err := s.API.GetTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
