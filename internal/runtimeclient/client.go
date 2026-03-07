package runtimeclient

import (
	"context"

	"github.com/dongowu/agentos/pkg/taskdsl"
)

// ExecutionResult represents the outcome of a single action.
type ExecutionResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// StreamChunk is a single stdout/stderr/metadata frame emitted while an action runs.
type StreamChunk struct {
	TaskID   string
	ActionID string
	Kind     string
	Data     []byte
}

// ExecutorClient sends actions to the Rust worker and receives structured results.
// Backed by protobuf/gRPC contracts.
type ExecutorClient interface {
	ExecuteAction(ctx context.Context, taskID string, action *taskdsl.Action) (*ExecutionResult, error)
}

// StreamingExecutorClient executes actions while forwarding stdout/stderr chunks.
type StreamingExecutorClient interface {
	ExecuteStream(ctx context.Context, taskID string, action *taskdsl.Action, sink func(StreamChunk)) (*ExecutionResult, error)
}
