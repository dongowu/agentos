package builtin

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"

	"github.com/dongowu/agentos/internal/tool"
)

func init() {
	tool.Register(&ShellTool{})
}

// ShellTool executes shell commands.
type ShellTool struct{}

func (ShellTool) Name() string        { return "shell" }
func (ShellTool) Description() string { return "Execute shell commands" }

func (ShellTool) Run(ctx context.Context, input map[string]any) (any, error) {
	cmdVal, ok := input["cmd"]
	if !ok {
		return nil, ErrMissingInput{Field: "cmd"}
	}
	cmdStr, ok := cmdVal.(string)
	if !ok {
		return nil, ErrInvalidInput{Field: "cmd", Want: "string"}
	}

	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.CommandContext(ctx, "cmd", "/c", cmdStr)
	default:
		c = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return map[string]any{
				"stdout":    stdout.String(),
				"stderr":    stderr.String(),
				"exit_code": ee.ExitCode(),
			}, nil
		}
		return nil, err
	}

	return map[string]any{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": 0,
	}, nil
}

type ErrMissingInput struct{ Field string }
type ErrInvalidInput struct{ Field, Want string }

func (e ErrMissingInput) Error() string  { return "shell: missing input: " + e.Field }
func (e ErrInvalidInput) Error() string { return "shell: invalid " + e.Field + ", want " + e.Want }
