package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dongowu/agentos/internal/access"
)

// HTTPTaskAPI is a thin remote adapter for the public task API.
type HTTPTaskAPI struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewHTTPTaskAPI returns an HTTP-backed task API client.
func NewHTTPTaskAPI(baseURL, token string) *HTTPTaskAPI {
	return &HTTPTaskAPI{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  http.DefaultClient,
	}
}

// WithClient overrides the HTTP client, primarily for tests.
func (a *HTTPTaskAPI) WithClient(client *http.Client) *HTTPTaskAPI {
	if client != nil {
		a.client = client
	}
	return a
}

// CreateTask implements access.TaskSubmissionAPI.
func (a *HTTPTaskAPI) CreateTask(ctx context.Context, req access.CreateTaskRequest) (*access.CreateTaskResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	var resp access.CreateTaskResponse
	if err := a.doJSON(ctx, http.MethodPost, "/v1/tasks", bytes.NewReader(body), "application/json", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetTask implements access.TaskSubmissionAPI.
func (a *HTTPTaskAPI) GetTask(ctx context.Context, taskID string) (*access.CreateTaskResponse, error) {
	var resp access.CreateTaskResponse
	if err := a.doJSON(ctx, http.MethodGet, "/v1/tasks/"+taskID, nil, "", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (a *HTTPTaskAPI) doJSON(ctx context.Context, method, path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	if len(payload) == 0 {
		return fmt.Errorf("server returned empty response")
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode response: %w: %s", err, string(payload))
	}
	return nil
}

var _ access.TaskSubmissionAPI = (*HTTPTaskAPI)(nil)
