package runtimeclient

import (
	"context"

	"github.com/agentos/agentos/pkg/taskdsl"
)

// StubExecutorClient returns a zero exit code for tests.
type StubExecutorClient struct{}

// ExecuteAction implements ExecutorClient.
func (c *StubExecutorClient) ExecuteAction(ctx context.Context, taskID string, action *taskdsl.Action) (*ExecutionResult, error) {
	return &ExecutionResult{ExitCode: 0, Stdout: []byte{}, Stderr: []byte{}}, nil
}
