package builtin

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

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