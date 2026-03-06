package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileReadTool_Name(t *testing.T) {
	tool := FileReadTool{}
	if tool.Name() != "file.read" {
		t.Fatalf("expected name file.read, got %q", tool.Name())
	}
}

func TestFileReadTool_Run_ReadsFileContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := FileReadTool{}
	out, err := tool.Run(context.Background(), map[string]any{"path": path})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out != "hello world" {
		t.Fatalf("expected 'hello world', got %q", out)
	}
}

func TestFileReadTool_Run_MissingPath(t *testing.T) {
	tool := FileReadTool{}
	_, err := tool.Run(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestFileReadTool_Run_InvalidPathType(t *testing.T) {
	tool := FileReadTool{}
	_, err := tool.Run(context.Background(), map[string]any{"path": 123})
	if err == nil {
		t.Fatal("expected error for non-string path")
	}
}

func TestFileReadTool_Run_FileNotFound(t *testing.T) {
	tool := FileReadTool{}
	_, err := tool.Run(context.Background(), map[string]any{"path": "/nonexistent/file.txt"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestFileWriteTool_Name(t *testing.T) {
	tool := FileWriteTool{}
	if tool.Name() != "file.write" {
		t.Fatalf("expected name file.write, got %q", tool.Name())
	}
}

func TestFileWriteTool_Run_WritesFileContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	tool := FileWriteTool{}
	out, err := tool.Run(context.Background(), map[string]any{"path": path, "content": "hello"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", out)
	}
	bytes, _ := result["bytes_written"].(int)
	if bytes != 5 {
		t.Fatalf("expected 5 bytes written, got %d", bytes)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected file to contain 'hello', got %q", string(data))
	}
}

func TestFileWriteTool_Run_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "out.txt")

	tool := FileWriteTool{}
	_, err := tool.Run(context.Background(), map[string]any{"path": path, "content": "nested"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "nested" {
		t.Fatalf("expected 'nested', got %q", string(data))
	}
}

func TestFileWriteTool_Run_MissingPath(t *testing.T) {
	tool := FileWriteTool{}
	_, err := tool.Run(context.Background(), map[string]any{"content": "hello"})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestFileWriteTool_Run_MissingContent(t *testing.T) {
	tool := FileWriteTool{}
	_, err := tool.Run(context.Background(), map[string]any{"path": "/tmp/test.txt"})
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}
