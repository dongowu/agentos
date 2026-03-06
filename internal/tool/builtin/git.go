package builtin

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/dongowu/agentos/internal/tool"
)

func init() {
	tool.Register(&GitCloneTool{})
	tool.Register(&GitStatusTool{})
}

// GitCloneTool clones a git repository.
type GitCloneTool struct{}

func (GitCloneTool) Name() string        { return "git.clone" }
func (GitCloneTool) Description() string { return "Clone a git repository" }

func (GitCloneTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "Repository URL to clone"},
			"dir": map[string]any{"type": "string", "description": "Target directory for clone"},
		},
		"required": []string{"url", "dir"},
	}
}

func (GitCloneTool) Run(ctx context.Context, input map[string]any) (any, error) {
	urlVal, ok := input["url"]
	if !ok {
		return nil, fmt.Errorf("git.clone: missing required input: url")
	}
	url, ok := urlVal.(string)
	if !ok {
		return nil, fmt.Errorf("git.clone: invalid input: url must be a string")
	}

	dirVal, ok := input["dir"]
	if !ok {
		return nil, fmt.Errorf("git.clone: missing required input: dir")
	}
	dir, ok := dirVal.(string)
	if !ok {
		return nil, fmt.Errorf("git.clone: invalid input: dir must be a string")
	}

	cmd := exec.CommandContext(ctx, "git", "clone", url, dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git.clone: %w: %s", err, string(output))
	}

	return map[string]any{
		"dir":    dir,
		"output": string(output),
	}, nil
}

// GitStatusTool returns the status of a git repository.
type GitStatusTool struct{}

func (GitStatusTool) Name() string        { return "git.status" }
func (GitStatusTool) Description() string { return "Get git repository status" }

func (GitStatusTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dir": map[string]any{"type": "string", "description": "Path to the git repository"},
		},
		"required": []string{"dir"},
	}
}

func (GitStatusTool) Run(ctx context.Context, input map[string]any) (any, error) {
	dirVal, ok := input["dir"]
	if !ok {
		return nil, fmt.Errorf("git.status: missing required input: dir")
	}
	dir, ok := dirVal.(string)
	if !ok {
		return nil, fmt.Errorf("git.status: invalid input: dir must be a string")
	}

	cmd := exec.CommandContext(ctx, "git", "-C", dir, "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git.status: %w: %s", err, string(output))
	}

	return map[string]any{
		"status": string(output),
	}, nil
}
