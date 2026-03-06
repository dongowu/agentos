package shell

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"

	"github.com/dongowu/agentos/internal/sandbox"
)

// Shell implements sandbox.Sandbox for shell command execution.
type Shell struct{}

// Type returns "shell".
func (Shell) Type() string { return "shell" }

// Run executes the shell command from spec.Command.
func (Shell) Run(ctx context.Context, spec sandbox.Spec) (sandbox.Result, error) {
	if spec.Command == "" {
		return sandbox.Result{ExitCode: -1}, ErrEmptyCommand{}
	}

	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.CommandContext(ctx, "cmd", "/c", spec.Command)
	default:
		c = exec.CommandContext(ctx, "sh", "-c", spec.Command)
	}

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	res := sandbox.Result{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: 0,
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
			return res, nil
		} else {
			res.ExitCode = -1
			return res, err
		}
	}
	return res, nil
}

// ErrEmptyCommand is returned when spec.Command is empty.
type ErrEmptyCommand struct{}

func (ErrEmptyCommand) Error() string { return "shell: empty command" }
