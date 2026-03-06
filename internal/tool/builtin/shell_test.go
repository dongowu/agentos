package builtin

import (
    "context"
    "runtime"
    "strings"
    "testing"

    "github.com/dongowu/agentos/internal/sandbox"
)

type fakeShellRunner struct {
    result sandbox.Result
    err    error
    gotCmd string
}

func (f *fakeShellRunner) Run(_ context.Context, spec sandbox.Spec) (sandbox.Result, error) {
    f.gotCmd = spec.Command
    return f.result, f.err
}

func TestShellTool_Run_DelegatesToSandboxRunner(t *testing.T) {
    runner := &fakeShellRunner{result: sandbox.Result{Stdout: []byte("out"), Stderr: []byte("err"), ExitCode: 7}}
    tool := ShellTool{Runner: runner}

    out, err := tool.Run(context.Background(), map[string]any{"cmd": "echo demo"})
    if err != nil {
        t.Fatalf("expected nil error, got %v", err)
    }
    if runner.gotCmd != "echo demo" {
        t.Fatalf("expected runner command echo demo, got %q", runner.gotCmd)
    }

    result, ok := out.(map[string]any)
    if !ok {
        t.Fatalf("expected map result, got %T", out)
    }
    if result["stdout"] != "out" || result["stderr"] != "err" || result["exit_code"] != 7 {
        t.Fatalf("unexpected mapped result: %#v", result)
    }
}

func TestShellTool_Run_ReturnsStructuredResultForNonZeroExit(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("shell behavior verification is deferred on Windows")
    }

    tool := ShellTool{}
    out, err := tool.Run(context.Background(), map[string]any{"cmd": "echo fail 1>&2; exit 7"})
    if err != nil {
        t.Fatalf("expected nil error for non-zero exit code, got %v", err)
    }

    result, ok := out.(map[string]any)
    if !ok {
        t.Fatalf("expected map result, got %T", out)
    }

    if result["exit_code"] != 7 {
        t.Fatalf("expected exit_code 7, got %v", result["exit_code"])
    }

    stderr, _ := result["stderr"].(string)
    if !strings.Contains(stderr, "fail") {
        t.Fatalf("expected stderr to contain fail, got %q", stderr)
    }
}