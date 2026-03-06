package runtimeclient

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "github.com/dongowu/agentos/api/gen/agentos/v1"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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
	payload, _ := json.Marshal(action.Payload)
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

// Close closes the gRPC connection.
func (c *GRPCExecutorClient) Close() error {
	return c.conn.Close()
}
