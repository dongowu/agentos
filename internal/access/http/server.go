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
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/events"
)

// Server exposes the HTTP API for task submission, agent run, and tool run.
type Server struct {
	Addr    string
	API     access.TaskSubmissionAPI
	Audit   persistence.AuditLogStore
	Bus     messaging.EventBus
	Auth    access.AuthProvider
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/tasks" {
		http.NotFound(w, r)
		return
	}
	ctx, _, ok := s.authenticateRequest(w, r)
	if !ok {
		return
	}
	r = r.WithContext(ctx)
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
	ctx, _, ok := s.authenticateRequest(w, r)
	if !ok {
		return
	}
	r = r.WithContext(ctx)
	path := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
	if path == "" {
		http.Error(w, `{"error":"task id required"}`, http.StatusBadRequest)
		return
	}
	if taskID, actionID, ok := parseActionStreamPath(path); ok {
		s.handleActionStream(w, r, taskID, actionID)
		return
	}
	if taskID, actionID, ok := parseActionAuditPath(path); ok {
		s.handleActionAudit(w, r, taskID, actionID)
		return
	}
	if taskID, ok := parseTaskStreamPath(path); ok {
		s.handleTaskStream(w, r, taskID)
		return
	}
	if taskID, ok := parseTaskAuditPath(path); ok {
		s.handleTaskAudit(w, r, taskID)
		return
	}
	s.handleGetTask(w, r, path)
}

func parseActionStreamPath(path string) (taskID, actionID string, ok bool) {
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[1] != "actions" || parts[3] != "stream" {
		return "", "", false
	}
	if parts[0] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[0], parts[2], true
}

func parseActionAuditPath(path string) (taskID, actionID string, ok bool) {
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[1] != "actions" || parts[3] != "audit" {
		return "", "", false
	}
	if parts[0] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[0], parts[2], true
}

func parseTaskStreamPath(path string) (taskID string, ok bool) {
	if !strings.HasSuffix(path, "/stream") {
		return "", false
	}
	taskID = strings.TrimSuffix(path, "/stream")
	taskID = strings.TrimSuffix(taskID, "/")
	if taskID == "" || strings.Contains(taskID, "/") {
		return "", false
	}
	return taskID, true
}

func parseTaskAuditPath(path string) (taskID string, ok bool) {
	if !strings.HasSuffix(path, "/audit") {
		return "", false
	}
	taskID = strings.TrimSuffix(path, "/audit")
	taskID = strings.TrimSuffix(taskID, "/")
	if taskID == "" || strings.Contains(taskID, "/") {
		return "", false
	}
	return taskID, true
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
	if principal, ok := access.PrincipalFromContext(r.Context()); ok {
		if req.TenantID == "" {
			req.TenantID = principal.TenantID
		} else if principal.TenantID != "" && req.TenantID != principal.TenantID {
			http.Error(w, `{"error":"tenant mismatch"}`, http.StatusForbidden)
			return
		}
	}
	resp, err := s.API.CreateTask(r.Context(), access.CreateTaskRequest{Prompt: req.Prompt, TenantID: req.TenantID, AgentName: req.AgentName})
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, resp)
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
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleTaskAudit(w http.ResponseWriter, r *http.Request, taskID string) {
	if s.API == nil {
		http.Error(w, `{"error":"api not configured"}`, http.StatusInternalServerError)
		return
	}
	if s.Audit == nil {
		http.Error(w, `{"error":"audit store not configured"}`, http.StatusInternalServerError)
		return
	}
	if _, err := s.API.GetTask(r.Context(), taskID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}
	records, err := s.Audit.ListByTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task_id": taskID, "records": records})
}

func (s *Server) handleActionAudit(w http.ResponseWriter, r *http.Request, taskID, actionID string) {
	if s.API == nil {
		http.Error(w, `{"error":"api not configured"}`, http.StatusInternalServerError)
		return
	}
	if s.Audit == nil {
		http.Error(w, `{"error":"audit store not configured"}`, http.StatusInternalServerError)
		return
	}
	if _, err := s.API.GetTask(r.Context(), taskID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}
	record, err := s.Audit.Get(r.Context(), taskID, actionID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if record == nil {
		http.Error(w, `{"error":"audit record not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, record)
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
	unsubscribers := make([]func(), 0, 5)
	for _, topic := range []string{"task.created", "task.planned", "task.action.dispatched", "task.action.output", "task.action.completed"} {
		topic := topic
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
		case event := <-eventsCh:
			if err := writeSSEEvent(w, event.topic, event.payload); err != nil {
				return
			}
			flusher.Flush()
		default:
		}
		select {
		case event := <-eventsCh:
			if err := writeSSEEvent(w, event.topic, event.payload); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleActionStream(w http.ResponseWriter, r *http.Request, taskID, actionID string) {
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
	if _, err := s.API.GetTask(r.Context(), taskID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	if s.Audit != nil {
		record, err := s.Audit.Get(r.Context(), taskID, actionID)
		if err == nil && record != nil {
			if record.Stdout != "" {
				if err := writeSSEEvent(w, "task.action.output", &events.ActionOutputChunk{TaskID: taskID, ActionID: actionID, Kind: "stdout", Data: []byte(record.Stdout), Text: record.Stdout, Occurred: record.OccurredAt}); err != nil {
					return
				}
			}
			if record.Stderr != "" {
				if err := writeSSEEvent(w, "task.action.output", &events.ActionOutputChunk{TaskID: taskID, ActionID: actionID, Kind: "stderr", Data: []byte(record.Stderr), Text: record.Stderr, Occurred: record.OccurredAt}); err != nil {
					return
				}
			}
			if err := writeSSEEvent(w, "task.action.completed", &events.ActionCompleted{TaskID: taskID, ActionID: actionID, ExitCode: record.ExitCode, WorkerID: record.WorkerID, Error: record.Error, Occurred: record.OccurredAt}); err != nil {
				return
			}
			flusher.Flush()
			return
		}
	}

	eventsCh := make(chan taskStreamEvent, 16)
	unsubscribers := make([]func(), 0, 2)
	for _, topic := range []string{"task.action.output", "task.action.completed"} {
		topic := topic
		unsub, err := s.Bus.Subscribe(topic, func(payload any) {
			if payloadTaskID(payload) != taskID || payloadActionID(payload) != actionID {
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
		case event := <-eventsCh:
			if err := writeSSEEvent(w, event.topic, event.payload); err != nil {
				return
			}
			flusher.Flush()
			if event.topic == "task.action.completed" {
				return
			}
		default:
		}
		select {
		case event := <-eventsCh:
			if err := writeSSEEvent(w, event.topic, event.payload); err != nil {
				return
			}
			flusher.Flush()
			if event.topic == "task.action.completed" {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) authenticateRequest(w http.ResponseWriter, r *http.Request) (context.Context, *access.Principal, bool) {
	if s.Auth == nil {
		return r.Context(), nil, true
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
		return nil, nil, false
	}
	principal, err := s.Auth.Authenticate(r.Context(), token)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return nil, nil, false
	}
	ctx := access.WithPrincipal(r.Context(), principal)
	return ctx, principal, true
}

func bearerToken(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}

func writeSSEEvent(w http.ResponseWriter, name string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
	return err
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func payloadTaskID(payload any) string {
	switch value := payload.(type) {
	case *events.TaskCreated:
		return value.TaskID
	case *events.TaskPlanned:
		return value.TaskID
	case *events.ActionDispatched:
		return value.TaskID
	case *events.ActionOutputChunk:
		return value.TaskID
	case *events.ActionCompleted:
		return value.TaskID
	case events.TaskCreated:
		return value.TaskID
	case events.TaskPlanned:
		return value.TaskID
	case events.ActionDispatched:
		return value.TaskID
	case events.ActionOutputChunk:
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

func payloadActionID(payload any) string {
	switch value := payload.(type) {
	case *events.ActionDispatched:
		return value.ActionID
	case *events.ActionOutputChunk:
		return value.ActionID
	case *events.ActionCompleted:
		return value.ActionID
	case events.ActionDispatched:
		return value.ActionID
	case events.ActionOutputChunk:
		return value.ActionID
	case events.ActionCompleted:
		return value.ActionID
	case map[string]any:
		for _, key := range []string{"action_id", "ActionID"} {
			if actionID, ok := value[key].(string); ok {
				return actionID
			}
		}
	}
	return ""
}
