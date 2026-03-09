package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/persistence"
)

type captureTaskAPI struct {
	lastCreate access.CreateTaskRequest
}

func (a *captureTaskAPI) CreateTask(_ context.Context, req access.CreateTaskRequest) (*access.CreateTaskResponse, error) {
	a.lastCreate = req
	return &access.CreateTaskResponse{TaskID: "task-123", State: "queued"}, nil
}

func (a *captureTaskAPI) GetTask(_ context.Context, taskID string) (*access.CreateTaskResponse, error) {
	return &access.CreateTaskResponse{TaskID: taskID, State: "running"}, nil
}

type stubAuthProvider struct {
	principal *access.Principal
	err       error
	lastToken string
}

func (a *stubAuthProvider) Authenticate(_ context.Context, token string) (*access.Principal, error) {
	a.lastToken = token
	if a.err != nil {
		return nil, a.err
	}
	return a.principal, nil
}

func TestServer_CreateTask_UsesAuthenticatedTenantWhenBodyOmitsTenant(t *testing.T) {
	api := &captureTaskAPI{}
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	srv := &Server{API: api, Auth: auth}

	body := bytes.NewBufferString(`{"prompt":"echo hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", body)
	req.Header.Set("Authorization", "Bearer token-123")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if auth.lastToken != "token-123" {
		t.Fatalf("expected bearer token token-123, got %q", auth.lastToken)
	}
	if api.lastCreate.TenantID != "tenant-auth" {
		t.Fatalf("expected tenant tenant-auth, got %q", api.lastCreate.TenantID)
	}
}

func TestServer_CreateTask_RejectsTenantMismatch(t *testing.T) {
	api := &captureTaskAPI{}
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	srv := &Server{API: api, Auth: auth}

	body := bytes.NewBufferString(`{"prompt":"echo hello","tenant_id":"tenant-other"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", body)
	req.Header.Set("Authorization", "Bearer token-123")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestServer_CreateTask_RequiresBearerTokenWhenAuthConfigured(t *testing.T) {
	srv := &Server{API: &captureTaskAPI{}, Auth: &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}}

	body := bytes.NewBufferString(`{"prompt":"echo hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestServer_GetTask_RequiresValidBearerTokenWhenAuthConfigured(t *testing.T) {
	srv := &Server{API: &captureTaskAPI{}, Auth: &stubAuthProvider{err: errors.New("bad token")}}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestServer_CreateTask_PreservesExplicitMatchingTenant(t *testing.T) {
	api := &captureTaskAPI{}
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	srv := &Server{API: api, Auth: auth}

	body := bytes.NewBufferString(`{"prompt":"echo hello","tenant_id":"tenant-auth"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", body)
	req.Header.Set("Authorization", "Bearer token-123")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp access.CreateTaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.TaskID != "task-123" {
		t.Fatalf("expected task-123, got %q", resp.TaskID)
	}
	if api.lastCreate.TenantID != "tenant-auth" {
		t.Fatalf("expected tenant-auth, got %q", api.lastCreate.TenantID)
	}
}

type tenantAwareTaskAPI struct {
	tenantID string
}

func (a *tenantAwareTaskAPI) CreateTask(_ context.Context, req access.CreateTaskRequest) (*access.CreateTaskResponse, error) {
	return &access.CreateTaskResponse{TaskID: "task-123", State: "queued"}, nil
}

func (a *tenantAwareTaskAPI) GetTask(_ context.Context, taskID string) (*access.CreateTaskResponse, error) {
	return &access.CreateTaskResponse{TaskID: taskID, State: "running"}, nil
}

func (a *tenantAwareTaskAPI) GetTaskDetail(_ context.Context, taskID string) (*access.TaskDetail, error) {
	return &access.TaskDetail{TaskID: taskID, State: "running", TenantID: a.tenantID}, nil
}

func TestServer_GetTask_RejectsCrossTenantRead(t *testing.T) {
	api := &tenantAwareTaskAPI{tenantID: "tenant-other"}
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	srv := &Server{API: api, Auth: auth}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestServer_TaskAudit_RejectsCrossTenantRead(t *testing.T) {
	api := &tenantAwareTaskAPI{tenantID: "tenant-other"}
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	audit := &stubAuditStore{}
	srv := &Server{API: api, Audit: audit, Auth: auth}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/audit", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestServer_TaskStream_RejectsCrossTenantRead(t *testing.T) {
	api := &tenantAwareTaskAPI{tenantID: "tenant-other"}
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	bus := newStreamBus()
	srv := &Server{API: api, Bus: bus, Auth: auth}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/stream", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestServer_ActionAudit_RejectsCrossTenantRead(t *testing.T) {
	api := &tenantAwareTaskAPI{tenantID: "tenant-other"}
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	audit := &stubAuditStore{}
	srv := &Server{API: api, Audit: audit, Auth: auth}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/actions/act-1/audit", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestServer_ActionStream_RejectsCrossTenantRead(t *testing.T) {
	api := &tenantAwareTaskAPI{tenantID: "tenant-other"}
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	bus := newStreamBus()
	srv := &Server{API: api, Bus: bus, Auth: auth}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/actions/act-1/stream", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

type tenantAwareReplayAPI struct {
	*tenantAwareTaskAPI
}

func (a *tenantAwareReplayAPI) GetTaskReplay(_ context.Context, taskID string) (*access.TaskReplay, error) {
	return &access.TaskReplay{TaskID: taskID, State: "running"}, nil
}

func TestServer_TaskReplay_RejectsCrossTenantRead(t *testing.T) {
	api := &tenantAwareReplayAPI{tenantAwareTaskAPI: &tenantAwareTaskAPI{tenantID: "tenant-other"}}
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	srv := &Server{API: api, Auth: auth}

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/replay", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestServer_GlobalAudit_UsesAuthenticatedTenantScope(t *testing.T) {
	auth := &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}
	audit := &stubAuditStore{global: []persistence.AuditRecord{
		{TaskID: "task-tenant-auth", ActionID: "act-1", TenantID: "tenant-auth", ExitCode: 0},
	}}
	srv := &Server{Audit: audit, Auth: auth}

	req := httptest.NewRequest(http.MethodGet, "/v1/audit?tenant_id=tenant-other", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp struct {
		Records []persistence.AuditRecord `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(resp.Records))
	}
	if resp.Records[0].TenantID != "tenant-auth" {
		t.Fatalf("expected tenant-auth, got %q", resp.Records[0].TenantID)
	}
}

func TestServer_ListAgents_RequiresBearerTokenWhenAuthConfigured(t *testing.T) {
	srv := &Server{Auth: &stubAuthProvider{principal: &access.Principal{Subject: "user-1", TenantID: "tenant-auth"}}}

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestServer_ListWorkers_RequiresValidBearerTokenWhenAuthConfigured(t *testing.T) {
	srv := &Server{Auth: &stubAuthProvider{err: errors.New("bad token")}}

	req := httptest.NewRequest(http.MethodGet, "/v1/workers", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	rec := httptest.NewRecorder()

	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}
