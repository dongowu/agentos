package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dongowu/agentos/internal/tool"
)

func init() {
	tool.Register(&FileReadTool{})
	tool.Register(&FileWriteTool{})
}

// FileReadTool reads file contents from disk.
type FileReadTool struct{}

func (FileReadTool) Name() string        { return "file.read" }
func (FileReadTool) Description() string { return "Read file contents from disk" }

func (FileReadTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Absolute path to the file to read"},
		},
		"required": []string{"path"},
	}
}

func (FileReadTool) Run(_ context.Context, input map[string]any) (any, error) {
	pathVal, ok := input["path"]
	if !ok {
		return nil, fmt.Errorf("file.read: missing required input: path")
	}
	path, ok := pathVal.(string)
	if !ok {
		return nil, fmt.Errorf("file.read: invalid input: path must be a string")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("file.read: %w", err)
	}
	return string(data), nil
}

// FileWriteTool writes content to a file on disk.
type FileWriteTool struct{}

func (FileWriteTool) Name() string        { return "file.write" }
func (FileWriteTool) Description() string { return "Write content to a file on disk" }

func (FileWriteTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Absolute path to the file to write"},
			"content": map[string]any{"type": "string", "description": "Content to write to the file"},
		},
		"required": []string{"path", "content"},
	}
}

func (FileWriteTool) Run(_ context.Context, input map[string]any) (any, error) {
	pathVal, ok := input["path"]
	if !ok {
		return nil, fmt.Errorf("file.write: missing required input: path")
	}
	path, ok := pathVal.(string)
	if !ok {
		return nil, fmt.Errorf("file.write: invalid input: path must be a string")
	}

	contentVal, ok := input["content"]
	if !ok {
		return nil, fmt.Errorf("file.write: missing required input: content")
	}
	content, ok := contentVal.(string)
	if !ok {
		return nil, fmt.Errorf("file.write: invalid input: content must be a string")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("file.write: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("file.write: %w", err)
	}

	return map[string]any{
		"bytes_written": len(content),
		"path":          path,
	}, nil
}
