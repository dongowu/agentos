package v1

import (
	"os"
	"strings"
	"testing"
)

func TestTaskProto_DefinesCoreMessages(t *testing.T) {
	data, err := os.ReadFile("task.proto")
	if err != nil {
		t.Fatalf("read task proto: %v", err)
	}

	content := string(data)
	for _, token := range []string{
		"message Task",
		"message Plan",
		"message Action",
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("missing token %q in task.proto", token)
		}
	}
}

func TestRuntimeProto_DefinesExecuteContracts(t *testing.T) {
	data, err := os.ReadFile("runtime.proto")
	if err != nil {
		t.Fatalf("read runtime proto: %v", err)
	}

	content := string(data)
	for _, token := range []string{
		"service RuntimeService",
		"message ExecuteActionRequest",
		"message ExecuteActionResponse",
		"message StreamChunk",
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("missing token %q in runtime.proto", token)
		}
	}
}
