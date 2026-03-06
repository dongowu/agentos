package runtimeclient

import (
	"context"
	"encoding/json"
	"testing"

	v1 "github.com/dongowu/agentos/api/gen/agentos/v1"
	intruntime "github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
	"google.golang.org/grpc"
)

type stubRuntimeServiceClient struct {
	lastReq *v1.ExecuteActionRequest
}

func (s *stubRuntimeServiceClient) ExecuteAction(_ context.Context, req *v1.ExecuteActionRequest, _ ...grpc.CallOption) (*v1.ExecuteActionResponse, error) {
	s.lastReq = req
	return &v1.ExecuteActionResponse{ExitCode: 0}, nil
}

func (s *stubRuntimeServiceClient) StreamOutput(_ context.Context, _ *v1.StreamOutputRequest, _ ...grpc.CallOption) (v1.RuntimeService_StreamOutputClient, error) {
	return nil, nil
}

func TestGRPCExecutorClient_NormalizesCmdPayloadToCommand(t *testing.T) {
	stub := &stubRuntimeServiceClient{}
	client := &GRPCExecutorClient{client: stub}

	_, err := client.ExecuteAction(context.Background(), "task-1", &taskdsl.Action{
		ID:         "action-1",
		RuntimeEnv: "default",
		Payload:    map[string]any{"cmd": "echo hello"},
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}

	if stub.lastReq == nil {
		t.Fatal("expected request to be sent")
	}

	var payload map[string]any
	if err := json.Unmarshal(stub.lastReq.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["command"]; got != "echo hello" {
		t.Fatalf("expected command=echo hello, got %#v", got)
	}
}

var _ intruntime.ExecutorClient = (*GRPCExecutorClient)(nil)
