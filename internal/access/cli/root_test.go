package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dongowu/agentos/internal/access"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type stubTaskAPI struct {
	createResp *access.CreateTaskResponse
	getResp    *access.CreateTaskResponse
}

func (s stubTaskAPI) CreateTask(_ context.Context, req access.CreateTaskRequest) (*access.CreateTaskResponse, error) {
	if s.createResp != nil {
		return s.createResp, nil
	}
	return &access.CreateTaskResponse{TaskID: "task-123", State: "queued"}, nil
}

func (s stubTaskAPI) GetTask(_ context.Context, taskID string) (*access.CreateTaskResponse, error) {
	if s.getResp != nil {
		return s.getResp, nil
	}
	return &access.CreateTaskResponse{TaskID: taskID, State: "running"}, nil
}

func TestHTTPTaskAPI_CreateTask_SendsBearerToken(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.String() != "http://agentos.test/v1/tasks" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("expected bearer token, got %q", got)
		}
		var payload access.CreateTaskRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Prompt != "echo hello" {
			t.Fatalf("expected prompt echo hello, got %q", payload.Prompt)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"task_id":"task-123","state":"queued"}`)),
		}, nil
	})}
	api := NewHTTPTaskAPI("http://agentos.test", "token-123").WithClient(client)

	resp, err := api.CreateTask(context.Background(), access.CreateTaskRequest{Prompt: "echo hello"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if resp.TaskID != "task-123" {
		t.Fatalf("expected task-123, got %q", resp.TaskID)
	}
}

func TestHTTPTaskAPI_GetTask_ReadsRemoteStatus(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != "http://agentos.test/v1/tasks/task-123" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"task_id":"task-123","state":"succeeded"}`)),
		}, nil
	})}
	api := NewHTTPTaskAPI("http://agentos.test", "").WithClient(client)

	resp, err := api.GetTask(context.Background(), "task-123")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if resp.State != "succeeded" {
		t.Fatalf("expected succeeded, got %q", resp.State)
	}
}

func TestRoot_RemoteMode_DoesNotInvokeLocalFactory(t *testing.T) {
	localCalls := 0
	remoteCalls := 0
	var remoteServer string
	var remoteToken string
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			remoteCalls++
			remoteServer = serverURL
			remoteToken = token
			return stubTaskAPI{createResp: &access.CreateTaskResponse{TaskID: "task-999", State: "queued"}}
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "--token", "token-123", "submit", "echo hello"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	if remoteCalls != 1 {
		t.Fatalf("expected one remote factory call, got %d", remoteCalls)
	}
	if remoteServer != "http://agentos.test" {
		t.Fatalf("expected remote server http://agentos.test, got %q", remoteServer)
	}
	if remoteToken != "token-123" {
		t.Fatalf("expected token-123, got %q", remoteToken)
	}
	if !strings.Contains(stdout.String(), "task task-999 created") {
		t.Fatalf("expected submit output, got %q", stdout.String())
	}
}
