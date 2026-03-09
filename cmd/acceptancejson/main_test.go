package main

import (
	"runtime"
	"testing"
)

func TestTaskAuditCount(t *testing.T) {
	count, err := taskAuditCount([]byte(`{"records":[{"action_id":"prompt-1"},{"action_id":"prompt-2"}]}`))
	if err != nil {
		t.Fatalf("taskAuditCount returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
}

func TestTaskAuditLastActionID(t *testing.T) {
	actionID, err := taskAuditLastActionID([]byte(`{"records":[{"action_id":"prompt-1"},{"action_id":"prompt-2"}]}`))
	if err != nil {
		t.Fatalf("taskAuditLastActionID returned error: %v", err)
	}
	if actionID != "prompt-2" {
		t.Fatalf("expected last action id prompt-2, got %q", actionID)
	}
}

func TestActionAuditExitCode(t *testing.T) {
	exitCode, err := actionAuditExitCode([]byte(`{"exit_code":0}`))
	if err != nil {
		t.Fatalf("actionAuditExitCode returned error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}

func TestGlobalAuditMatchCount(t *testing.T) {
	count, err := globalAuditMatchCount([]byte(`{"records":[{"task_id":"task-1","tenant_id":"tenant-a"},{"task_id":"task-2","tenant_id":"tenant-a"},{"task_id":"task-1","tenant_id":"tenant-b"}]}`), "task-1", "tenant-a")
	if err != nil {
		t.Fatalf("globalAuditMatchCount returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}
}

func TestTaskAuditLastWorkerID(t *testing.T) {
	workerID, err := taskAuditLastWorkerID([]byte(`{"records":[{"worker_id":"worker-a"},{"worker_id":"control-plane"}]}`))
	if err != nil {
		t.Fatalf("taskAuditLastWorkerID returned error: %v", err)
	}
	if workerID != "control-plane" {
		t.Fatalf("expected worker id control-plane, got %q", workerID)
	}
}

func TestNativePath(t *testing.T) {
	t.Run("windows tmp path", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("windows-only path normalization")
		}
		got, err := nativePath(`/tmp/agentos-acceptance/bridge.txt`)
		if err != nil {
			t.Fatalf("nativePath returned error: %v", err)
		}
		if got == `/tmp/agentos-acceptance/bridge.txt` {
			t.Fatalf("expected native Windows path, got unchanged %q", got)
		}
	})

	t.Run("absolute path preserved", func(t *testing.T) {
		input := t.TempDir()
		got, err := nativePath(input)
		if err != nil {
			t.Fatalf("nativePath returned error: %v", err)
		}
		if got != input {
			t.Fatalf("expected %q, got %q", input, got)
		}
	})
}

func TestCreateTaskRequestBody(t *testing.T) {
	body, err := createTaskRequestBody(`write hello to C:\temp\bridge.txt then read C:\temp\bridge.txt`)
	if err != nil {
		t.Fatalf("createTaskRequestBody returned error: %v", err)
	}
	if body != `{"prompt":"write hello to C:\\temp\\bridge.txt then read C:\\temp\\bridge.txt"}` {
		t.Fatalf("unexpected body: %s", body)
	}
}
