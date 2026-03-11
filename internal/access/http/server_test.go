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

type stubHealthWorkerRegistry struct {
	workers []worker.WorkerInfo
	err     error
}

func (s stubHealthWorkerRegistry) List(_ context.Context) ([]worker.WorkerInfo, error) {
	return append([]worker.WorkerInfo(nil), s.workers...), s.err
}

func (s stubHealthWorkerRegistry) GetAvailable(_ context.Context) ([]worker.WorkerInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]worker.WorkerInfo, 0, len(s.workers))
	for _, info := range s.workers {
		if info.Status != worker.StatusOnline {
			continue
		}
		if info.MaxTasks > 0 && info.ActiveTasks >= info.MaxTasks {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

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
	srv := &Server{
		API:                      stubTaskAPI{},
		SchedulerMode:            "nats",
		SchedulerRecoveryEnabled: true,
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-1", Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
			{ID: "worker-2", Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
			{ID: "worker-3", Status: worker.StatusOffline, ActiveTasks: 0, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Status          string   `json:"status"`
		SchedulerMode   string   `json:"scheduler_mode"`
		RecoveryEnabled bool     `json:"recovery_enabled"`
		DegradedReasons []string `json:"degraded_reasons"`
		Workers         struct {
			Total            int `json:"total"`
			Online           int `json:"online"`
			Busy             int `json:"busy"`
			Offline          int `json:"offline"`
			AvailableWorkers int `json:"available_workers"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
	if resp.SchedulerMode != "nats" {
		t.Fatalf("expected scheduler mode nats, got %q", resp.SchedulerMode)
	}
	if !resp.RecoveryEnabled {
		t.Fatal("expected recovery_enabled true")
	}
	if len(resp.DegradedReasons) != 0 {
		t.Fatalf("expected no degraded reasons, got %v", resp.DegradedReasons)
	}
	if resp.Workers.Total != 3 || resp.Workers.Online != 1 || resp.Workers.Busy != 1 || resp.Workers.Offline != 1 || resp.Workers.AvailableWorkers != 1 {
		t.Fatalf("unexpected worker summary: %+v", resp.Workers)
	}
}

func TestServer_Handler_HealthRoute_DegradedWhenNoWorkersAvailable(t *testing.T) {
	srv := &Server{
		API:           stubTaskAPI{},
		SchedulerMode: "local",
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-1", Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
			{ID: "worker-2", Status: worker.StatusOffline, ActiveTasks: 0, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Status          string   `json:"status"`
		DegradedReasons []string `json:"degraded_reasons"`
		Workers         struct {
			AvailableWorkers int `json:"available_workers"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "degraded" {
		t.Fatalf("expected degraded status, got %q", resp.Status)
	}
	if resp.Workers.AvailableWorkers != 0 {
		t.Fatalf("expected 0 available workers, got %d", resp.Workers.AvailableWorkers)
	}
	if len(resp.DegradedReasons) != 1 || resp.DegradedReasons[0] != "no available workers" {
		t.Fatalf("unexpected degraded reasons: %v", resp.DegradedReasons)
	}
}

func TestServer_Handler_HealthRoute_IncludesCapabilityWarnings(t *testing.T) {
	srv := &Server{
		API:           stubTaskAPI{},
		SchedulerMode: "local",
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-1", Capabilities: []string{"native"}, Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
			{ID: "worker-2", Capabilities: []string{"docker"}, Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Status           string   `json:"status"`
		CapacityWarnings []string `json:"capacity_warnings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
	if len(resp.CapacityWarnings) != 1 || resp.CapacityWarnings[0] != "no available workers for capability docker" {
		t.Fatalf("unexpected capacity warnings: %v", resp.CapacityWarnings)
	}
}

func TestServer_Handler_ReadyRoute(t *testing.T) {
	srv := &Server{
		API:                      stubTaskAPI{},
		SchedulerMode:            "nats",
		SchedulerRecoveryEnabled: true,
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-1", Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
			{ID: "worker-2", Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Status          string   `json:"status"`
		SchedulerMode   string   `json:"scheduler_mode"`
		RecoveryEnabled bool     `json:"recovery_enabled"`
		DegradedReasons []string `json:"degraded_reasons"`
		Workers         struct {
			AvailableWorkers int `json:"available_workers"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
	if resp.SchedulerMode != "nats" {
		t.Fatalf("expected scheduler mode nats, got %q", resp.SchedulerMode)
	}
	if !resp.RecoveryEnabled {
		t.Fatal("expected recovery_enabled true")
	}
	if len(resp.DegradedReasons) != 0 {
		t.Fatalf("expected no degraded reasons, got %v", resp.DegradedReasons)
	}
	if resp.Workers.AvailableWorkers != 1 {
		t.Fatalf("expected 1 available worker, got %d", resp.Workers.AvailableWorkers)
	}
}

func TestServer_Handler_ReadyRoute_IncludesCapabilityWarningsWithoutFlippingReadiness(t *testing.T) {
	srv := &Server{
		API:           stubTaskAPI{},
		SchedulerMode: "nats",
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-1", Capabilities: []string{"native"}, Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
			{ID: "worker-2", Capabilities: []string{"docker"}, Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Status           string   `json:"status"`
		CapacityWarnings []string `json:"capacity_warnings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
	if len(resp.CapacityWarnings) != 1 || resp.CapacityWarnings[0] != "no available workers for capability docker" {
		t.Fatalf("unexpected capacity warnings: %v", resp.CapacityWarnings)
	}
}

func TestServer_Handler_ReadyRoute_DegradedWhenNoWorkersAvailable(t *testing.T) {
	srv := &Server{
		API:           stubTaskAPI{},
		SchedulerMode: "local",
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-1", Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
			{ID: "worker-2", Status: worker.StatusOffline, ActiveTasks: 0, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var resp struct {
		Status          string   `json:"status"`
		DegradedReasons []string `json:"degraded_reasons"`
		Workers         struct {
			AvailableWorkers int `json:"available_workers"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "degraded" {
		t.Fatalf("expected degraded status, got %q", resp.Status)
	}
	if resp.Workers.AvailableWorkers != 0 {
		t.Fatalf("expected 0 available workers, got %d", resp.Workers.AvailableWorkers)
	}
	if len(resp.DegradedReasons) != 1 || resp.DegradedReasons[0] != "no available workers" {
		t.Fatalf("unexpected degraded reasons: %v", resp.DegradedReasons)
	}
}

func TestServer_Handler_ReadyRoute_RemainsPublicWhenAuthConfigured(t *testing.T) {
	srv := &Server{
		API:            stubTaskAPI{},
		Auth:           &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}},
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{{ID: "worker-1", Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 1}}},
	}

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestServer_Handler_WorkersRoute(t *testing.T) {
	srv := &Server{
		API:  stubTaskAPI{},
		Auth: &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}},
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-2", Addr: "127.0.0.1:5002", Capabilities: []string{"docker"}, Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
			{ID: "worker-1", Addr: "127.0.0.1:5001", Capabilities: []string{"native"}, Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/workers", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Summary struct {
			Total            int `json:"total"`
			Online           int `json:"online"`
			Busy             int `json:"busy"`
			Offline          int `json:"offline"`
			AvailableWorkers int `json:"available_workers"`
		} `json:"summary"`
		Workers []struct {
			ID           string   `json:"id"`
			Addr         string   `json:"addr"`
			Capabilities []string `json:"capabilities"`
			Status       string   `json:"status"`
			ActiveTasks  int      `json:"active_tasks"`
			MaxTasks     int      `json:"max_tasks"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Summary.Total != 2 || resp.Summary.Online != 1 || resp.Summary.Busy != 1 || resp.Summary.Offline != 0 || resp.Summary.AvailableWorkers != 1 {
		t.Fatalf("unexpected summary: %+v", resp.Summary)
	}
	if len(resp.Workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(resp.Workers))
	}
	if resp.Workers[0].ID != "worker-1" || resp.Workers[1].ID != "worker-2" {
		t.Fatalf("expected workers sorted by id, got %+v", resp.Workers)
	}
	if resp.Workers[0].Addr != "127.0.0.1:5001" || resp.Workers[0].Status != "online" || resp.Workers[0].ActiveTasks != 0 || resp.Workers[0].MaxTasks != 2 {
		t.Fatalf("unexpected first worker payload: %+v", resp.Workers[0])
	}
	if len(resp.Workers[0].Capabilities) != 1 || resp.Workers[0].Capabilities[0] != "native" {
		t.Fatalf("unexpected first worker capabilities: %+v", resp.Workers[0].Capabilities)
	}
}

func TestServer_Handler_WorkersRoute_RegistryUnavailable(t *testing.T) {
	srv := &Server{
		API:            stubTaskAPI{},
		Auth:           &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}},
		WorkerRegistry: stubHealthWorkerRegistry{err: context.DeadlineExceeded},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/workers", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestServer_Handler_WorkersRoute_AvailableOnly(t *testing.T) {
	srv := &Server{
		API:  stubTaskAPI{},
		Auth: &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}},
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-3", Addr: "127.0.0.1:5003", Status: worker.StatusOffline, ActiveTasks: 0, MaxTasks: 2},
			{ID: "worker-2", Addr: "127.0.0.1:5002", Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
			{ID: "worker-1", Addr: "127.0.0.1:5001", Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/workers?available_only=true", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Summary struct {
			Total            int `json:"total"`
			Online           int `json:"online"`
			Busy             int `json:"busy"`
			Offline          int `json:"offline"`
			AvailableWorkers int `json:"available_workers"`
		} `json:"summary"`
		Workers []struct {
			ID string `json:"id"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Summary.Total != 1 || resp.Summary.Online != 1 || resp.Summary.Busy != 0 || resp.Summary.Offline != 0 || resp.Summary.AvailableWorkers != 1 {
		t.Fatalf("unexpected filtered summary: %+v", resp.Summary)
	}
	if len(resp.Workers) != 1 || resp.Workers[0].ID != "worker-1" {
		t.Fatalf("expected only worker-1, got %+v", resp.Workers)
	}
}

func TestServer_Handler_WorkersRoute_StatusAndCapabilityFilters(t *testing.T) {
	srv := &Server{
		API:  stubTaskAPI{},
		Auth: &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}},
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-4", Addr: "127.0.0.1:5004", Capabilities: []string{"native"}, Status: worker.StatusOffline, ActiveTasks: 0, MaxTasks: 2},
			{ID: "worker-3", Addr: "127.0.0.1:5003", Capabilities: []string{"docker"}, Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
			{ID: "worker-2", Addr: "127.0.0.1:5002", Capabilities: []string{"native"}, Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
			{ID: "worker-1", Addr: "127.0.0.1:5001", Capabilities: []string{"native"}, Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/workers?status=online&capability=native", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Summary struct {
			Total            int `json:"total"`
			Online           int `json:"online"`
			Busy             int `json:"busy"`
			Offline          int `json:"offline"`
			AvailableWorkers int `json:"available_workers"`
		} `json:"summary"`
		Workers []struct {
			ID           string   `json:"id"`
			Status       string   `json:"status"`
			Capabilities []string `json:"capabilities"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Summary.Total != 1 || resp.Summary.Online != 1 || resp.Summary.Busy != 0 || resp.Summary.Offline != 0 || resp.Summary.AvailableWorkers != 1 {
		t.Fatalf("unexpected filtered summary: %+v", resp.Summary)
	}
	if len(resp.Workers) != 1 || resp.Workers[0].ID != "worker-1" {
		t.Fatalf("expected only worker-1, got %+v", resp.Workers)
	}
	if resp.Workers[0].Status != "online" {
		t.Fatalf("expected worker-1 to remain online, got %+v", resp.Workers[0])
	}
	if len(resp.Workers[0].Capabilities) != 1 || resp.Workers[0].Capabilities[0] != "native" {
		t.Fatalf("unexpected worker capabilities: %+v", resp.Workers[0].Capabilities)
	}
}

func TestServer_Handler_WorkersRoute_IncludesCapabilitySummary(t *testing.T) {
	srv := &Server{
		API:  stubTaskAPI{},
		Auth: &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}},
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{
			{ID: "worker-3", Addr: "127.0.0.1:5003", Capabilities: []string{"docker"}, Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
			{ID: "worker-2", Addr: "127.0.0.1:5002", Capabilities: []string{"native"}, Status: worker.StatusBusy, ActiveTasks: 2, MaxTasks: 2},
			{ID: "worker-1", Addr: "127.0.0.1:5001", Capabilities: []string{"native"}, Status: worker.StatusOnline, ActiveTasks: 0, MaxTasks: 2},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/workers", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Summary struct {
			Capabilities []struct {
				Name             string `json:"name"`
				Total            int    `json:"total"`
				Online           int    `json:"online"`
				Busy             int    `json:"busy"`
				Offline          int    `json:"offline"`
				AvailableWorkers int    `json:"available_workers"`
			} `json:"capabilities"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Summary.Capabilities) != 2 {
		t.Fatalf("expected 2 capability summaries, got %+v", resp.Summary.Capabilities)
	}
	if resp.Summary.Capabilities[0].Name != "docker" || resp.Summary.Capabilities[0].Total != 1 || resp.Summary.Capabilities[0].Online != 1 || resp.Summary.Capabilities[0].Busy != 0 || resp.Summary.Capabilities[0].Offline != 0 || resp.Summary.Capabilities[0].AvailableWorkers != 1 {
		t.Fatalf("unexpected docker capability summary: %+v", resp.Summary.Capabilities[0])
	}
	if resp.Summary.Capabilities[1].Name != "native" || resp.Summary.Capabilities[1].Total != 2 || resp.Summary.Capabilities[1].Online != 1 || resp.Summary.Capabilities[1].Busy != 1 || resp.Summary.Capabilities[1].Offline != 0 || resp.Summary.Capabilities[1].AvailableWorkers != 1 {
		t.Fatalf("unexpected native capability summary: %+v", resp.Summary.Capabilities[1])
	}
}

func TestServer_Handler_WorkersRoute_RejectsInvalidAvailableOnlyQuery(t *testing.T) {
	srv := &Server{
		API:            stubTaskAPI{},
		Auth:           &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}},
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{{ID: "worker-1", Status: worker.StatusOnline, MaxTasks: 1}}},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/workers?available_only=maybe", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestServer_Handler_WorkersRoute_RequiresBearerTokenWhenAuthConfigured(t *testing.T) {
	srv := &Server{
		API:            stubTaskAPI{},
		Auth:           &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}},
		WorkerRegistry: stubHealthWorkerRegistry{workers: []worker.WorkerInfo{{ID: "worker-1", Status: worker.StatusOnline, MaxTasks: 1}}},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/workers", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
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
