package builtin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dongowu/agentos/internal/tool"
)

func init() {
	tool.Register(&HTTPGetTool{})
	tool.Register(&HTTPPostTool{})
}

// HTTPGetTool performs HTTP GET requests.
type HTTPGetTool struct{}

func (HTTPGetTool) Name() string        { return "http.get" }
func (HTTPGetTool) Description() string { return "Perform an HTTP GET request" }

func (HTTPGetTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "URL to send the GET request to"},
		},
		"required": []string{"url"},
	}
}

func (HTTPGetTool) Run(ctx context.Context, input map[string]any) (any, error) {
	urlVal, ok := input["url"]
	if !ok {
		return nil, fmt.Errorf("http.get: missing required input: url")
	}
	url, ok := urlVal.(string)
	if !ok {
		return nil, fmt.Errorf("http.get: invalid input: url must be a string")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http.get: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http.get: reading response: %w", err)
	}

	return map[string]any{
		"status": resp.StatusCode,
		"body":   string(body),
	}, nil
}

// HTTPPostTool performs HTTP POST requests.
type HTTPPostTool struct{}

func (HTTPPostTool) Name() string        { return "http.post" }
func (HTTPPostTool) Description() string { return "Perform an HTTP POST request" }

func (HTTPPostTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":          map[string]any{"type": "string", "description": "URL to send the POST request to"},
			"body":         map[string]any{"type": "string", "description": "Request body content"},
			"content_type": map[string]any{"type": "string", "description": "Content-Type header (default: application/json)"},
		},
		"required": []string{"url", "body"},
	}
}

func (HTTPPostTool) Run(ctx context.Context, input map[string]any) (any, error) {
	urlVal, ok := input["url"]
	if !ok {
		return nil, fmt.Errorf("http.post: missing required input: url")
	}
	url, ok := urlVal.(string)
	if !ok {
		return nil, fmt.Errorf("http.post: invalid input: url must be a string")
	}

	bodyVal, ok := input["body"]
	if !ok {
		return nil, fmt.Errorf("http.post: missing required input: body")
	}
	body, ok := bodyVal.(string)
	if !ok {
		return nil, fmt.Errorf("http.post: invalid input: body must be a string")
	}

	contentType := "application/json"
	if ctVal, ok := input["content_type"]; ok {
		if ct, ok := ctVal.(string); ok {
			contentType = ct
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("http.post: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http.post: reading response: %w", err)
	}

	return map[string]any{
		"status": resp.StatusCode,
		"body":   string(respBody),
	}, nil
}
