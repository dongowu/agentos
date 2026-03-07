package runtimeclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	v1 "github.com/dongowu/agentos/api/gen/agentos/v1"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func normalizeActionPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	normalized := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		normalized[key] = value
	}
	if _, hasCommand := normalized["command"]; !hasCommand {
		if cmd, hasCmd := normalized["cmd"]; hasCmd {
			normalized["command"] = cmd
		}
	}
	return normalized
}

// GRPCExecutorClient implements runtimeclient.ExecutorClient via gRPC.
type GRPCExecutorClient struct {
	addr   string
	client v1.RuntimeServiceClient
	conn   *grpc.ClientConn
}

// NewGRPCExecutorClient dials the worker and returns a client.
func NewGRPCExecutorClient(ctx context.Context, addr string) (*GRPCExecutorClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}
	return &GRPCExecutorClient{
		addr:   addr,
		client: v1.NewRuntimeServiceClient(conn),
		conn:   conn,
	}, nil
}

// ExecuteAction implements runtimeclient.ExecutorClient.
func (c *GRPCExecutorClient) ExecuteAction(ctx context.Context, taskID string, action *taskdsl.Action) (*runtimeclient.ExecutionResult, error) {
	payload, _ := json.Marshal(normalizeActionPayload(action.Payload))
	req := &v1.ExecuteActionRequest{
		TaskId:         taskID,
		ActionId:       action.ID,
		RuntimeProfile: action.RuntimeEnv,
		Payload:        payload,
	}
	resp, err := c.client.ExecuteAction(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute action: %w", err)
	}
	return &runtimeclient.ExecutionResult{
		ExitCode: int(resp.ExitCode),
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
	}, nil
}

// ExecuteStream implements runtimeclient.StreamingExecutorClient.
func (c *GRPCExecutorClient) ExecuteStream(ctx context.Context, taskID string, action *taskdsl.Action, sink func(runtimeclient.StreamChunk)) (*runtimeclient.ExecutionResult, error) {
	payload, _ := json.Marshal(normalizeActionPayload(action.Payload))
	stream, err := c.client.StreamOutput(ctx, &v1.StreamOutputRequest{
		TaskId:   taskID,
		ActionId: string(payload),
	})
	if err != nil {
		return nil, fmt.Errorf("stream output: %w", err)
	}

	result := &runtimeclient.ExecutionResult{}
	seenExit := false
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("receive stream chunk: %w", err)
		}

		normalized := runtimeclient.StreamChunk{
			TaskID:   taskID,
			ActionID: action.ID,
			Kind:     chunk.GetKind(),
			Data:     append([]byte(nil), chunk.GetData()...),
		}
		switch chunk.GetKind() {
		case "stdout":
			result.Stdout = append(result.Stdout, chunk.GetData()...)
			if sink != nil {
				sink(normalized)
			}
		case "stderr":
			result.Stderr = append(result.Stderr, chunk.GetData()...)
			if sink != nil {
				sink(normalized)
			}
		case "exit":
			exitCode, parseErr := strconv.Atoi(string(chunk.GetData()))
			if parseErr != nil {
				return nil, fmt.Errorf("parse exit code: %w", parseErr)
			}
			result.ExitCode = exitCode
			seenExit = true
		case "error":
			return nil, fmt.Errorf("stream output: %s", string(chunk.GetData()))
		default:
			if sink != nil {
				sink(normalized)
			}
		}
	}
	if !seenExit {
		return nil, fmt.Errorf("stream output ended without exit chunk")
	}
	return result, nil
}

// Close closes the gRPC connection.
func (c *GRPCExecutorClient) Close() error {
	return c.conn.Close()
}
