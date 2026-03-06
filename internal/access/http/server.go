package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dongowu/ai-orchestrator/internal/access"
)

// Server exposes the HTTP API for task submission and streaming.
type Server struct {
	Addr string
	API  access.TaskSubmissionAPI
	srv  *http.Server
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
	mux.HandleFunc("/v1/tasks", s.handleTasks)
	mux.HandleFunc("/v1/tasks/", s.handleTaskByID)
	return mux
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
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Prompt == "" {
		http.Error(w, `{"error":"prompt required"}`, http.StatusBadRequest)
		return
	}
	resp, err := s.API.CreateTask(r.Context(), access.CreateTaskRequest{Prompt: req.Prompt})
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
