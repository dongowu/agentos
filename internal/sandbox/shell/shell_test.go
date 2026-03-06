package shell

import (
    "context"
    "runtime"
    "strings"
    "testing"

    "github.com/dongowu/agentos/internal/sandbox"
)

func TestShellRun_ReturnsStructuredResultForNonZeroExit(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("shell behavior verification is deferred on Windows")
    }

    s := Shell{}
    result, err := s.Run(context.Background(), sandbox.Spec{Command: "echo out; echo err 1>&2; exit 7"})
    if err != nil {
        t.Fatalf("expected nil error for non-zero exit code, got %v", err)
    }
    if result.ExitCode != 7 {
        t.Fatalf("expected exit code 7, got %d", result.ExitCode)
    }
    if !strings.Contains(string(result.Stdout), "out") {
        t.Fatalf("expected stdout to contain out, got %q", string(result.Stdout))
    }
    if !strings.Contains(string(result.Stderr), "err") {
        t.Fatalf("expected stderr to contain err, got %q", string(result.Stderr))
    }
}