package actionbridge

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	_ "github.com/dongowu/agentos/internal/tool/builtin"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

func TestBridge_ExecuteFileWriteAndRead(t *testing.T) {
	bridge := New()
	path := filepath.Join(t.TempDir(), "out.txt")

	writeResult, err := bridge.Execute(context.Background(), "task-1", &taskdsl.Action{ID: "a1", Kind: "file.write", Payload: map[string]any{"path": path, "content": "hello"}}, nil)
	if err != nil {
		t.Fatalf("Execute write: %v", err)
	}
	if writeResult.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", writeResult.ExitCode)
	}
	var writePayload map[string]any
	if err := json.Unmarshal(writeResult.Stdout, &writePayload); err != nil {
		t.Fatalf("unmarshal write result: %v", err)
	}
	if writePayload["path"] != path {
		t.Fatalf("unexpected write payload: %#v", writePayload)
	}

	readResult, err := bridge.Execute(context.Background(), "task-1", &taskdsl.Action{ID: "a2", Kind: "file.read", Payload: map[string]any{"path": path}}, nil)
	if err != nil {
		t.Fatalf("Execute read: %v", err)
	}
	if string(readResult.Stdout) != "hello" {
		t.Fatalf("expected hello, got %q", readResult.Stdout)
	}
}

func TestBridge_HTTPRequestRejectsUnsupportedMethod(t *testing.T) {
	bridge := New()
	_, err := bridge.Execute(context.Background(), "task-1", &taskdsl.Action{ID: "a1", Kind: "http.request", Payload: map[string]any{"method": "DELETE", "url": "https://example.com"}}, nil)
	if err == nil {
		t.Fatal("expected unsupported method error")
	}
}
