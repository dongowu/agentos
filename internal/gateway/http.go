package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/tool"
)

// Handler provides ClawOS API: /agent/run, /tool/run.
type Handler struct {
	TaskAPI access.TaskSubmissionAPI
}

// NewHandler returns a gateway handler.
func NewHandler(taskAPI access.TaskSubmissionAPI) *Handler {
	return &Handler{TaskAPI: taskAPI}
}

// AgentRunRequest is the body for POST /agent/run.
type AgentRunRequest struct {
	Agent string `json:"agent"`
	Task  string `json:"task"`
}

// AgentRunResponse is the response for POST /agent/run.
type AgentRunResponse struct {
	TaskID string `json:"task_id"`
	State  string `json:"state"`
}

// ToolRunRequest is the body for POST /tool/run.
type ToolRunRequest struct {
	Tool  string         `json:"tool"`
	Input map[string]any `json:"input"`
}

// ServeAgentRun handles POST /agent/run.
func (h *Handler) ServeAgentRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req AgentRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Task == "" {
		writeJSONError(w, http.StatusBadRequest, "task required")
		return
	}

	// MVP: use task as prompt. Agent name used for future routing.
	prompt := req.Task
	if req.Agent != "" {
		// Optional: load agent config for model/memory/tools (future)
		_ = req.Agent
	}

	if h.TaskAPI == nil {
		writeJSONError(w, http.StatusInternalServerError, "task api not configured")
		return
	}
	resp, err := h.TaskAPI.CreateTask(r.Context(), access.CreateTaskRequest{Prompt: prompt})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AgentRunResponse{TaskID: resp.TaskID, State: resp.State})
}

// ServeAgentStatus handles GET /agent/status?task_id=...
func (h *Handler) ServeAgentStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		writeJSONError(w, http.StatusBadRequest, "task_id required")
		return
	}
	if h.TaskAPI == nil {
		writeJSONError(w, http.StatusInternalServerError, "task api not configured")
		return
	}

	resp, err := h.TaskAPI.GetTask(r.Context(), taskID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AgentRunResponse{TaskID: resp.TaskID, State: resp.State})
}

// ServeToolRun handles POST /tool/run.
func (h *Handler) ServeToolRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req ToolRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Tool == "" {
		writeJSONError(w, http.StatusBadRequest, "tool required")
		return
	}
	if req.Input == nil {
		req.Input = make(map[string]any)
	}

	out, err := tool.Run(r.Context(), req.Tool, req.Input)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"result": out})
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
