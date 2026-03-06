package builtin

import (
	"context"

	"github.com/dongowu/agentos/internal/sandbox"
	shellsandbox "github.com/dongowu/agentos/internal/sandbox/shell"
	"github.com/dongowu/agentos/internal/tool"
)

func init() {
	tool.Register(&ShellTool{})
}

// ShellTool executes shell commands.
type ShellTool struct {
	Runner shellRunner
}

type shellRunner interface {
	Run(ctx context.Context, spec sandbox.Spec) (sandbox.Result, error)
}

func (ShellTool) Name() string        { return "shell" }
func (ShellTool) Description() string { return "Execute shell commands" }

func (s ShellTool) Run(ctx context.Context, input map[string]any) (any, error) {
	cmdVal, ok := input["cmd"]
	if !ok {
		return nil, ErrMissingInput{Field: "cmd"}
	}
	cmdStr, ok := cmdVal.(string)
	if !ok {
		return nil, ErrInvalidInput{Field: "cmd", Want: "string"}
	}

	runner := s.Runner
	if runner == nil {
		runner = shellsandbox.Shell{}
	}

	result, err := runner.Run(ctx, sandbox.Spec{Type: "shell", Command: cmdStr})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"stdout":    string(result.Stdout),
		"stderr":    string(result.Stderr),
		"exit_code": result.ExitCode,
	}, nil
}

type ErrMissingInput struct{ Field string }
type ErrInvalidInput struct{ Field, Want string }

func (e ErrMissingInput) Error() string  { return "shell: missing input: " + e.Field }
func (e ErrInvalidInput) Error() string { return "shell: invalid " + e.Field + ", want " + e.Want }
