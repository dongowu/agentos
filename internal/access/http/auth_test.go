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
