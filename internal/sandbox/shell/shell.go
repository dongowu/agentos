package shell

import (
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

	out, err := c.CombinedOutput()
	res := sandbox.Result{
		Stdout:   out,
		Stderr:   nil,
		ExitCode: 0,
	}
	if err != nil {
		res.Stderr = out
		res.Stdout = nil
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
		}
	}
	return res, nil
}

// ErrEmptyCommand is returned when spec.Command is empty.
type ErrEmptyCommand struct{}

func (ErrEmptyCommand) Error() string { return "shell: empty command" }
