package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
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

func TestHTTPTaskAPI_ListWorkers_SendsBearerTokenAndAvailableOnlyQuery(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != "http://agentos.test/v1/workers?available_only=true" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("expected bearer token, got %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}
	api := NewHTTPTaskAPI("http://agentos.test", "token-123").WithClient(client)

	resp, err := api.ListWorkers(context.Background(), WorkerListFilters{AvailableOnly: true})
	if err != nil {
		t.Fatalf("ListWorkers: %v", err)
	}
	if resp.Summary.AvailableWorkers != 1 || len(resp.Workers) != 1 || resp.Workers[0].ID != "worker-1" {
		t.Fatalf("unexpected worker response: %+v", resp)
	}
}

func assertIndentedJSONEqual(t *testing.T, got string, want string) {
	t.Helper()

	var gotValue any
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("expected valid JSON, got %q: %v", got, err)
	}
	var wantValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("expected valid want JSON, got %q: %v", want, err)
	}
	if reflect.DeepEqual(gotValue, wantValue) {
		return
	}
	var gotBuf bytes.Buffer
	_ = json.Indent(&gotBuf, []byte(got), "", "  ")
	var wantBuf bytes.Buffer
	_ = json.Indent(&wantBuf, []byte(want), "", "  ")
	t.Fatalf("unexpected JSON output:\n got: %s\nwant: %s", gotBuf.String(), wantBuf.String())
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

func TestRoot_WorkersCommand_RemoteMode_PrintsSummaryAndWorkers(t *testing.T) {
	localCalls := 0
	var authHeader string
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		authHeader = req.Header.Get("Authorization")
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != "http://agentos.test/v1/workers?available_only=true" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "--token", "token-123", "workers", "--available"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	body := stdout.String()
	if !strings.Contains(body, "summary: total=1 online=1 busy=0 offline=0 available=1") {
		t.Fatalf("expected summary output, got %q", body)
	}
	if !strings.Contains(body, "ID") || !strings.Contains(body, "STATUS") || !strings.Contains(body, "CAPABILITIES") {
		t.Fatalf("expected table headers, got %q", body)
	}
	if !strings.Contains(body, "worker-1") || !strings.Contains(body, "127.0.0.1:5001") || !strings.Contains(body, "native") {
		t.Fatalf("expected worker table row, got %q", body)
	}
	if authHeader != "Bearer token-123" {
		t.Fatalf("expected bearer token, got %q", authHeader)
	}
}

func TestRoot_WorkersCommand_RemoteMode_ThreadsStatusAndCapabilityFilters(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/workers" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("available_only") != "true" {
			t.Fatalf("expected available_only=true, got %q", query.Get("available_only"))
		}
		if query.Get("status") != "online" {
			t.Fatalf("expected status=online, got %q", query.Get("status"))
		}
		if query.Get("capability") != "native" {
			t.Fatalf("expected capability=native, got %q", query.Get("capability"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "--token", "token-123", "workers", "--available", "--status", "online", "--capability", "native"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RemoteMode_PrintsCapabilitySummary(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":3,
					"online":2,
					"busy":1,
					"offline":0,
					"available_workers":2,
					"capabilities":[
						{"name":"docker","total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
						{"name":"native","total":2,"online":1,"busy":1,"offline":0,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["native"]},
					{"id":"worker-3","addr":"127.0.0.1:5003","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["docker"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	body := stdout.String()
	if !strings.Contains(body, "capability docker: total=1 online=1 busy=0 offline=0 available=1") {
		t.Fatalf("expected docker capability summary, got %q", body)
	}
	if !strings.Contains(body, "capability native: total=2 online=1 busy=1 offline=0 available=1") {
		t.Fatalf("expected native capability summary, got %q", body)
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputJSON(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":1,
					"online":1,
					"busy":0,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"native","total":1,"online":1,"busy":0,"offline":0,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	var resp WorkerListResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", stdout.String(), err)
	}
	if resp.Summary.Total != 1 || len(resp.Workers) != 1 || resp.Workers[0].ID != "worker-1" {
		t.Fatalf("unexpected JSON output: %+v", resp)
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputJSON_IncludesSchemaVersion(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":1,
					"online":1,
					"busy":0,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"native","total":1,"online":1,"busy":0,"offline":0,"available_workers":1}
					]
				},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", stdout.String(), err)
	}
	var schema string
	if err := json.Unmarshal(payload["schema_version"], &schema); err != nil {
		t.Fatalf("expected schema_version string, got %+v: %v", payload, err)
	}
	if schema != "v1" {
		t.Fatalf("expected schema_version v1, got %q", schema)
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputJSON_ContractStabilizesCapabilityOrder(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":2,
					"online":2,
					"busy":0,
					"offline":0,
					"available_workers":2,
					"capabilities":[
						{"name":"shell","total":2,"online":2,"busy":0,"offline":0,"available_workers":2},
						{"name":"native","total":1,"online":1,"busy":0,"offline":0,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell","native"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json", "--sort", "id"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}

	assertIndentedJSONEqual(t, stdout.String(), `{
	  "schema_version": "v1",
	  "summary": {
	    "total": 2,
	    "online": 2,
	    "busy": 0,
	    "offline": 0,
	    "available_workers": 2,
	    "capabilities": [
	      {
	        "name": "native",
	        "total": 1,
	        "online": 1,
	        "busy": 0,
	        "offline": 0,
	        "available_workers": 1
	      },
	      {
	        "name": "shell",
	        "total": 2,
	        "online": 2,
	        "busy": 0,
	        "offline": 0,
	        "available_workers": 2
	      }
	    ]
	  },
	  "workers": [
	    {
	      "id": "worker-1",
	      "addr": "127.0.0.1:5001",
	      "capabilities": [
	        "native",
	        "shell"
	      ],
	      "status": "online",
	      "active_tasks": 0,
	      "max_tasks": 2
	    },
	    {
	      "id": "worker-2",
	      "addr": "127.0.0.1:5002",
	      "capabilities": [
	        "shell"
	      ],
	      "status": "online",
	      "active_tasks": 0,
	      "max_tasks": 2
	    }
	  ]
	}`)
}

func TestRoot_WorkersCommand_RemoteMode_OutputJSON_ContractSummaryOnly(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":2,
					"online":1,
					"busy":1,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"shell","total":2,"online":1,"busy":1,"offline":0,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["shell"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json", "--summary-only"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}

	assertIndentedJSONEqual(t, stdout.String(), `{
	  "schema_version": "v1",
	  "summary": {
	    "total": 2,
	    "online": 1,
	    "busy": 1,
	    "offline": 0,
	    "available_workers": 1,
	    "capabilities": [
	      {
	        "name": "shell",
	        "total": 2,
	        "online": 1,
	        "busy": 1,
	        "offline": 0,
	        "available_workers": 1
	      }
	    ]
	  }
	}`)
}

func TestRoot_WorkersCommand_RemoteMode_OutputJSON_ContractWorkersOnly(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":2,
					"online":1,
					"busy":1,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"shell","total":2,"online":1,"busy":1,"offline":0,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell","native"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json", "--workers-only", "--sort", "id"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}

	assertIndentedJSONEqual(t, stdout.String(), `{
	  "schema_version": "v1",
	  "workers": [
	    {
	      "id": "worker-1",
	      "addr": "127.0.0.1:5001",
	      "capabilities": [
	        "native",
	        "shell"
	      ],
	      "status": "online",
	      "active_tasks": 0,
	      "max_tasks": 2
	    },
	    {
	      "id": "worker-2",
	      "addr": "127.0.0.1:5002",
	      "capabilities": [
	        "shell"
	      ],
	      "status": "busy",
	      "active_tasks": 2,
	      "max_tasks": 2
	    }
	  ]
	}`)
}

func TestRoot_WorkersCommand_RemoteMode_OutputTableFlag(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "table"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	if !strings.Contains(stdout.String(), "summary: total=1 online=1 busy=0 offline=0 available=1") {
		t.Fatalf("expected table output, got %q", stdout.String())
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputTableUsesColumnHeaders(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":1,
					"online":1,
					"busy":0,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"native","total":1,"online":1,"busy":0,"offline":0,"available_workers":1}
					]
				},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "table"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	body := stdout.String()
	if !strings.Contains(body, "ID") || !strings.Contains(body, "STATUS") || !strings.Contains(body, "CAPABILITIES") {
		t.Fatalf("expected table headers, got %q", body)
	}
	if !strings.Contains(body, "CAPABILITY") || !strings.Contains(body, "AVAILABLE") {
		t.Fatalf("expected capability summary table headers, got %q", body)
	}
	if strings.Contains(body, "worker-1 online 0/2 127.0.0.1:5001 capabilities=native") {
		t.Fatalf("expected columnar table output, got old free-form row %q", body)
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputTable_CanHideCapabilitySummary(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":1,
					"online":1,
					"busy":0,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"native","total":1,"online":1,"busy":0,"offline":0,"available_workers":1}
					]
				},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "table", "--no-capability-summary"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	body := stdout.String()
	if strings.Contains(body, "CAPABILITY") || strings.Contains(body, "AVAILABLE") {
		t.Fatalf("expected capability summary table omitted, got %q", body)
	}
	if strings.Contains(body, "capability native:") {
		t.Fatalf("expected capability summary omitted, got %q", body)
	}
	if !strings.Contains(body, "ID") || !strings.Contains(body, "worker-1") {
		t.Fatalf("expected worker table retained, got %q", body)
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputTable_CanHideWorkers(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":1,
					"online":1,
					"busy":0,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"native","total":1,"online":1,"busy":0,"offline":0,"available_workers":1}
					]
				},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "table", "--no-workers"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	body := stdout.String()
	if !strings.Contains(body, "CAPABILITY") || !strings.Contains(body, "native") {
		t.Fatalf("expected capability summary table retained, got %q", body)
	}
	if strings.Contains(body, "ID") || strings.Contains(body, "worker-1") {
		t.Fatalf("expected worker table omitted, got %q", body)
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputTable_UnschedulableOnly(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":3,
					"online":2,
					"busy":1,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"native","total":3,"online":2,"busy":1,"offline":0,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["native"]},
					{"id":"worker-3","addr":"127.0.0.1:5003","status":"offline","active_tasks":0,"max_tasks":2,"capabilities":["native"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "table", "--unschedulable-only"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	body := stdout.String()
	if !strings.Contains(body, "summary: total=2 online=0 busy=1 offline=1 available=0") {
		t.Fatalf("expected recomputed summary, got %q", body)
	}
	if strings.Contains(body, "worker-1") {
		t.Fatalf("expected schedulable worker omitted, got %q", body)
	}
	if !strings.Contains(body, "worker-2") || !strings.Contains(body, "worker-3") {
		t.Fatalf("expected unschedulable workers retained, got %q", body)
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputJSON_SortLoadAndLimit(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":3,
					"online":2,
					"busy":1,
					"offline":0,
					"available_workers":2,
					"capabilities":[
						{"name":"native","total":3,"online":2,"busy":1,"offline":0,"available_workers":2}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":1,"max_tasks":2,"capabilities":["native"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["native"]},
					{"id":"worker-3","addr":"127.0.0.1:5003","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json", "--sort", "load", "--limit", "2"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	var payload workerListJSONEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", stdout.String(), err)
	}
	if payload.Summary == nil || payload.Summary.Total != 2 {
		t.Fatalf("expected limited summary total=2, got %+v", payload.Summary)
	}
	if len(payload.Workers) != 2 {
		t.Fatalf("expected 2 workers, got %+v", payload.Workers)
	}
	if payload.Workers[0].ID != "worker-2" || payload.Workers[1].ID != "worker-1" {
		t.Fatalf("expected load-sorted workers [worker-2 worker-1], got %+v", payload.Workers)
	}
}

func TestRoot_WorkersCommand_RejectsUnknownSortMode(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--sort", "queue"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported sort mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireCountFails(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-count", "2"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "worker count requirement failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireWorkerFailsWhenMissing(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-worker", "worker-1,worker-2"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "required workers missing: worker-2") {
		t.Fatalf("unexpected error: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireCapabilityCountFailsWhenMissing(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-capability-count", "native=1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "capability count requirement failed: native expected at least 1, got 0") {
		t.Fatalf("unexpected error: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireCapabilityCountPassesAfterFiltering(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":2,
					"online":2,
					"busy":0,
					"offline":0,
					"available_workers":2,
					"capabilities":[
						{"name":"native","total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
						{"name":"shell","total":2,"online":2,"busy":0,"offline":0,"available_workers":2}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell","native"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--sort", "id", "--limit", "1", "--require-capability-count", "shell=1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RejectsInvalidRequireCapabilityCount(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-capability-count", "shell"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "capability count values must use capability=count") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireCapabilityAvailableCountFailsWhenMissing(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":2,
					"online":1,
					"busy":1,
					"offline":0,
					"available_workers":0,
					"capabilities":[
						{"name":"shell","total":2,"online":1,"busy":1,"offline":0,"available_workers":0}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":2,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["shell"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-capability-available-count", "shell=1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "capability available count requirement failed: shell expected at least 1, got 0") {
		t.Fatalf("unexpected error: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireCapabilityAvailableCountPasses(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":2,
					"online":2,
					"busy":0,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"native","total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
						{"name":"shell","total":2,"online":2,"busy":0,"offline":0,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell","native"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"online","active_tasks":2,"max_tasks":2,"capabilities":["shell"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-capability-available-count", "shell=1,native=1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RejectsInvalidRequireCapabilityAvailableCount(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-capability-available-count", "shell"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "capability count values must use capability=count") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireCapabilityOnlineCountFailsWhenMissing(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":2,
					"online":1,
					"busy":1,
					"offline":0,
					"available_workers":1,
					"capabilities":[
						{"name":"shell","total":2,"online":1,"busy":1,"offline":0,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["shell"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-capability-online-count", "shell=2"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "capability online count requirement failed: shell expected at least 2, got 1") {
		t.Fatalf("unexpected error: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireCapabilityStatusCountsPass(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":4,
					"online":2,
					"busy":1,
					"offline":1,
					"available_workers":2,
					"capabilities":[
						{"name":"native","total":2,"online":1,"busy":0,"offline":1,"available_workers":1},
						{"name":"shell","total":3,"online":1,"busy":1,"offline":1,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-3","addr":"127.0.0.1:5003","status":"offline","active_tasks":0,"max_tasks":2,"capabilities":["shell","native"]},
					{"id":"worker-4","addr":"127.0.0.1:5004","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{
		"--server", "http://agentos.test",
		"workers",
		"--require-capability-online-count", "native=1,shell=1",
		"--require-capability-busy-count", "shell=1",
		"--require-capability-offline-count", "shell=1,native=1",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RejectsInvalidRequireCapabilityOnlineCount(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-capability-online-count", "shell"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "capability count values must use capability=count") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireStatusCountFailsWhenMissing(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-status-count", "offline=1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status count requirement failed: offline expected at least 1, got 0") {
		t.Fatalf("unexpected error: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireStatusCountPassesAfterFiltering(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{
					"total":3,
					"online":1,
					"busy":1,
					"offline":1,
					"available_workers":1,
					"capabilities":[
						{"name":"shell","total":3,"online":1,"busy":1,"offline":1,"available_workers":1}
					]
				},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-3","addr":"127.0.0.1:5003","status":"offline","active_tasks":0,"max_tasks":2,"capabilities":["shell"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--unschedulable-only", "--require-status-count", "offline=1,busy=1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RejectsInvalidRequireStatusCountStatus(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-status-count", "degraded=1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported worker status requirement") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireAvailableCountFailsWhenMissing(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":2,"online":1,"busy":0,"offline":1,"available_workers":0},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"offline","active_tasks":0,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"online","active_tasks":2,"max_tasks":2,"capabilities":["shell"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-available-count", "1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "available worker count requirement failed: expected at least 1, got 0") {
		t.Fatalf("unexpected error: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireAvailableCountPassesAfterFiltering(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":3,"online":2,"busy":1,"offline":0,"available_workers":1},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"busy","active_tasks":2,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-3","addr":"127.0.0.1:5003","status":"online","active_tasks":2,"max_tasks":2,"capabilities":["shell"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--limit", "2", "--sort", "id", "--require-available-count", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RejectsNegativeRequireAvailableCount(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-available-count", "-1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "require-available-count must be >= 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireLoadThresholdFailsWhenExceeded(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":2,"online":2,"busy":0,"offline":0,"available_workers":2},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":1,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"online","active_tasks":2,"max_tasks":2,"capabilities":["shell"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-load-threshold", "0.75"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "load threshold requirement failed: worker worker-2 load 1.00 exceeds 0.75") {
		t.Fatalf("unexpected error: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RemoteMode_RequireLoadThresholdPassesAfterFiltering(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":2,"online":2,"busy":0,"offline":0,"available_workers":2},
				"workers":[
					{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["shell"]},
					{"id":"worker-2","addr":"127.0.0.1:5002","status":"online","active_tasks":2,"max_tasks":2,"capabilities":["shell"]}
				]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--sort", "id", "--limit", "1", "--require-load-threshold", "0.10"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}

func TestRoot_WorkersCommand_RejectsInvalidRequireLoadThreshold(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--require-load-threshold", "-0.1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "require-load-threshold must be a float >= 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RejectsTableTrimFlagsForJSONOutput(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json", "--no-workers"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no-capability-summary and no-workers require --output table") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputJSON_SummaryOnly(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json", "--summary-only"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", stdout.String(), err)
	}
	if _, ok := payload["summary"]; !ok {
		t.Fatalf("expected summary key, got %+v", payload)
	}
	if _, ok := payload["workers"]; ok {
		t.Fatalf("expected workers key omitted, got %+v", payload)
	}
	if _, ok := payload["schema_version"]; !ok {
		t.Fatalf("expected schema_version key, got %+v", payload)
	}
}

func TestRoot_WorkersCommand_RemoteMode_OutputJSON_WorkersOnly(t *testing.T) {
	localCalls := 0
	server := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"summary":{"total":1,"online":1,"busy":0,"offline":0,"available_workers":1},
				"workers":[{"id":"worker-1","addr":"127.0.0.1:5001","status":"online","active_tasks":0,"max_tasks":2,"capabilities":["native"]}]
			}`)),
		}, nil
	})}

	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token).WithClient(server)
		},
	)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json", "--workers-only"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %v", stdout.String(), err)
	}
	if _, ok := payload["workers"]; !ok {
		t.Fatalf("expected workers key, got %+v", payload)
	}
	if _, ok := payload["summary"]; ok {
		t.Fatalf("expected summary key omitted, got %+v", payload)
	}
	if _, ok := payload["schema_version"]; !ok {
		t.Fatalf("expected schema_version key, got %+v", payload)
	}
}

func TestRoot_WorkersCommand_RejectsConflictingJSONTrimFlags(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "json", "--summary-only", "--workers-only"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "summary-only and workers-only are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RejectsUnknownOutputMode(t *testing.T) {
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
		func(serverURL, token string) WorkerAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--server", "http://agentos.test", "workers", "--output", "yaml"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_WorkersCommand_RequiresServer(t *testing.T) {
	localCalls := 0
	root := newRoot(
		func() (access.TaskSubmissionAPI, error) {
			localCalls++
			return stubTaskAPI{}, nil
		},
		func(serverURL, token string) access.TaskSubmissionAPI {
			return NewHTTPTaskAPI(serverURL, token)
		},
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"workers"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "workers command requires --server") {
		t.Fatalf("unexpected error: %v", err)
	}
	if localCalls != 0 {
		t.Fatalf("expected local factory to remain unused, got %d calls", localCalls)
	}
}
