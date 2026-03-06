package builtin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}
	return dir
}

func initRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestGitCloneTool_Name(t *testing.T) {
	tool := GitCloneTool{}
	if tool.Name() != "git.clone" {
		t.Fatalf("expected name git.clone, got %q", tool.Name())
	}
}

func TestGitCloneTool_Run_ClonesLocalRepo(t *testing.T) {
	src := initRepoWithCommit(t)
	dst := filepath.Join(t.TempDir(), "clone")

	tool := GitCloneTool{}
	out, err := tool.Run(context.Background(), map[string]any{"url": src, "dir": dst})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", out)
	}
	if result["dir"] != dst {
		t.Fatalf("expected dir %q, got %q", dst, result["dir"])
	}

	// Verify the clone worked
	if _, err := os.Stat(filepath.Join(dst, ".git")); os.IsNotExist(err) {
		t.Fatal("expected .git directory in clone target")
	}
}

func TestGitCloneTool_Run_MissingURL(t *testing.T) {
	tool := GitCloneTool{}
	_, err := tool.Run(context.Background(), map[string]any{"dir": "/tmp/test"})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestGitCloneTool_Run_MissingDir(t *testing.T) {
	tool := GitCloneTool{}
	_, err := tool.Run(context.Background(), map[string]any{"url": "https://example.com/repo"})
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestGitStatusTool_Name(t *testing.T) {
	tool := GitStatusTool{}
	if tool.Name() != "git.status" {
		t.Fatalf("expected name git.status, got %q", tool.Name())
	}
}

func TestGitStatusTool_Run_CleanRepo(t *testing.T) {
	dir := initRepoWithCommit(t)

	tool := GitStatusTool{}
	out, err := tool.Run(context.Background(), map[string]any{"dir": dir})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", out)
	}
	status, _ := result["status"].(string)
	if strings.Contains(status, "Changes") {
		t.Fatalf("expected clean status, got %q", status)
	}
}

func TestGitStatusTool_Run_DirtyRepo(t *testing.T) {
	dir := initRepoWithCommit(t)

	// Create an untracked file
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := GitStatusTool{}
	out, err := tool.Run(context.Background(), map[string]any{"dir": dir})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", out)
	}
	status, _ := result["status"].(string)
	if !strings.Contains(status, "new.txt") {
		t.Fatalf("expected status to mention new.txt, got %q", status)
	}
}

func TestGitStatusTool_Run_MissingDir(t *testing.T) {
	tool := GitStatusTool{}
	_, err := tool.Run(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestGitStatusTool_Run_NotARepo(t *testing.T) {
	dir := t.TempDir()

	tool := GitStatusTool{}
	_, err := tool.Run(context.Background(), map[string]any{"dir": dir})
	if err == nil {
		t.Fatal("expected error for non-repo directory")
	}
}
