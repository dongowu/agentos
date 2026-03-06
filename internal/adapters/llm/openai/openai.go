package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dongowu/agentos/internal/adapters/llm"
)

// Client is an OpenAI-compatible chat completion client using net/http.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a Client for the given base URL and API key.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// chatRequest is the JSON body sent to the completions endpoint.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse mirrors the OpenAI chat completion response shape.
type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

// Chat sends a chat completion request and returns the assistant's response.
func (c *Client) Chat(ctx context.Context, req llm.Request) (*llm.Response, error) {
	messages := make([]chatMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = chatMessage{Role: m.Role, Content: m.Content}
	}

	body := chatRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: API returned status %d", httpResp.StatusCode)
	}

	var resp chatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices in response")
	}

	return &llm.Response{Content: resp.Choices[0].Message.Content}, nil
}
