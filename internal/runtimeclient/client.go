package runtimeclient

import (
	"context"

	"github.com/dongowu/ai-orchestrator/pkg/taskdsl"
)

// ExecutionResult represents the outcome of a single action.
type ExecutionResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// ExecutorClient sends actions to the Rust worker and receives structured results.
// Backed by protobuf/gRPC contracts.
type ExecutorClient interface {
	ExecuteAction(ctx context.Context, taskID string, action *taskdsl.Action) (*ExecutionResult, error)
}
