package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/gateway"
	"github.com/dongowu/agentos/internal/messaging"
	"github.com/dongowu/agentos/pkg/events"
)

// Server exposes the HTTP API for task submission, agent run, and tool run.
type Server struct {
	Addr    string
	API     access.TaskSubmissionAPI
	Bus     messaging.EventBus
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
	path := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
	if path == "" {
		http.Error(w, `{"error":"task id required"}`, http.StatusBadRequest)
		return
	}
	if strings.HasSuffix(path, "/stream") {
		taskID := strings.TrimSuffix(path, "/stream")
		taskID = strings.TrimSuffix(taskID, "/")
		if taskID == "" {
			http.Error(w, `{"error":"task id required"}`, http.StatusBadRequest)
			return
		}
		s.handleTaskStream(w, r, taskID)
		return
	}
	s.handleGetTask(w, r, path)
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

type taskStreamEvent struct {
	topic   string
	payload any
}

func (s *Server) handleTaskStream(w http.ResponseWriter, r *http.Request, taskID string) {
	if s.API == nil {
		http.Error(w, `{"error":"api not configured"}`, http.StatusInternalServerError)
		return
	}
	if s.Bus == nil {
		http.Error(w, `{"error":"event bus not configured"}`, http.StatusInternalServerError)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming unsupported"}`, http.StatusInternalServerError)
		return
	}
	state, err := s.API.GetTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if err := writeSSEEvent(w, "task.snapshot", state); err != nil {
		return
	}
	flusher.Flush()

	eventsCh := make(chan taskStreamEvent, 32)
	unsubscribers := make([]func(), 0, 4)
	for _, topic := range []string{"task.created", "task.planned", "task.action.dispatched", "task.action.completed"} {
		unsub, err := s.Bus.Subscribe(topic, func(payload any) {
			if payloadTaskID(payload) != taskID {
				return
			}
			select {
			case eventsCh <- taskStreamEvent{topic: topic, payload: payload}:
			default:
			}
		})
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		unsubscribers = append(unsubscribers, unsub)
	}
	defer func() {
		for _, unsub := range unsubscribers {
			unsub()
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-eventsCh:
			if err := writeSSEEvent(w, event.topic, event.payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, name string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
	return err
}

func payloadTaskID(payload any) string {
	switch value := payload.(type) {
	case *events.TaskCreated:
		return value.TaskID
	case *events.TaskPlanned:
		return value.TaskID
	case *events.ActionDispatched:
		return value.TaskID
	case *events.ActionCompleted:
		return value.TaskID
	case events.TaskCreated:
		return value.TaskID
	case events.TaskPlanned:
		return value.TaskID
	case events.ActionDispatched:
		return value.TaskID
	case events.ActionCompleted:
		return value.TaskID
	case map[string]any:
		for _, key := range []string{"task_id", "TaskID"} {
			if taskID, ok := value[key].(string); ok {
				return taskID
			}
		}
	}
	return ""
}
