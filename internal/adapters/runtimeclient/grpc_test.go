package runtimeclient

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	v1 "github.com/dongowu/agentos/api/gen/agentos/v1"
	intruntime "github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
	"google.golang.org/grpc"
)

type stubRuntimeServiceClient struct {
	lastReq       *v1.ExecuteActionRequest
	lastStreamReq *v1.StreamOutputRequest
	streamChunks  []*v1.StreamChunk
}

func (s *stubRuntimeServiceClient) ExecuteAction(_ context.Context, req *v1.ExecuteActionRequest, _ ...grpc.CallOption) (*v1.ExecuteActionResponse, error) {
	s.lastReq = req
	return &v1.ExecuteActionResponse{ExitCode: 0}, nil
}

func (s *stubRuntimeServiceClient) StreamOutput(_ context.Context, req *v1.StreamOutputRequest, _ ...grpc.CallOption) (v1.RuntimeService_StreamOutputClient, error) {
	s.lastStreamReq = req
	return &stubRuntimeStreamClient{chunks: s.streamChunks}, nil
}

type stubRuntimeStreamClient struct {
	grpc.ClientStream
	chunks []*v1.StreamChunk
	index  int
}

func (s *stubRuntimeStreamClient) Recv() (*v1.StreamChunk, error) {
	if s.index >= len(s.chunks) {
		return nil, io.EOF
	}
	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
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

func TestGRPCExecutorClient_ExecuteStream_UsesPayloadAndAggregatesChunks(t *testing.T) {
	stub := &stubRuntimeServiceClient{streamChunks: []*v1.StreamChunk{
		{TaskId: "task-1", ActionId: "ignored", Kind: "stdout", Data: []byte("hel")},
		{TaskId: "task-1", ActionId: "ignored", Kind: "stdout", Data: []byte("lo")},
		{TaskId: "task-1", ActionId: "ignored", Kind: "stderr", Data: []byte("warn")},
		{TaskId: "task-1", ActionId: "ignored", Kind: "exit", Data: []byte("7")},
	}}
	client := &GRPCExecutorClient{client: stub}

	var streamed []intruntime.StreamChunk
	result, err := client.ExecuteStream(context.Background(), "task-1", &taskdsl.Action{
		ID:         "action-1",
		RuntimeEnv: "default",
		Payload:    map[string]any{"cmd": "echo hello"},
	}, func(chunk intruntime.StreamChunk) {
		streamed = append(streamed, chunk)
	})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	if stub.lastStreamReq == nil {
		t.Fatal("expected stream request to be sent")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stub.lastStreamReq.ActionId), &payload); err != nil {
		t.Fatalf("unmarshal stream payload: %v", err)
	}
	if got := payload["command"]; got != "echo hello" {
		t.Fatalf("expected command=echo hello, got %#v", got)
	}

	if result.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", result.ExitCode)
	}
	if string(result.Stdout) != "hello" {
		t.Fatalf("expected stdout hello, got %q", result.Stdout)
	}
	if string(result.Stderr) != "warn" {
		t.Fatalf("expected stderr warn, got %q", result.Stderr)
	}
	if len(streamed) != 3 {
		t.Fatalf("expected 3 streamed chunks, got %d", len(streamed))
	}
	if streamed[0].ActionID != "action-1" {
		t.Fatalf("expected streamed action id action-1, got %q", streamed[0].ActionID)
	}
	if string(streamed[2].Data) != "warn" {
		t.Fatalf("expected final stderr chunk warn, got %q", streamed[2].Data)
	}
}

var _ intruntime.ExecutorClient = (*GRPCExecutorClient)(nil)
var _ intruntime.StreamingExecutorClient = (*GRPCExecutorClient)(nil)
