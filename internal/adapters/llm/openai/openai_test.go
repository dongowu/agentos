package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dongowu/agentos/internal/adapters/llm"
)

func TestClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path.
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		// Verify auth header.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", auth)
		}

		// Verify content type.
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}

		// Decode and verify the request body.
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["model"] != "gpt-4" {
			t.Errorf("expected model gpt-4, got %v", body["model"])
		}

		// Return a valid OpenAI-style response.
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello from mock LLM",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	resp, err := client.Chat(context.Background(), llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "user", Content: "hello"},
		},
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from mock LLM" {
		t.Errorf("expected 'Hello from mock LLM', got %q", resp.Content)
	}
}

func TestClient_Chat_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key")
	_, err := client.Chat(context.Background(), llm.Request{
		Model:    "gpt-4",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestClient_Chat_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"choices": []any{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	_, err := client.Chat(context.Background(), llm.Request{
		Model:    "gpt-4",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestClient_Chat_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response — the context will cancel before this completes.
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := NewClient(server.URL, "test-key")
	_, err := client.Chat(ctx, llm.Request{
		Model:    "gpt-4",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestClient_Chat_ToolCalling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}

		// Verify tools were sent
		tools, ok := body["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("expected 1 tool, got %v", body["tools"])
		}

		// Return a tool_calls response
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]any{
							{
								"id":   "call_abc123",
								"type": "function",
								"function": map[string]any{
									"name":      "shell",
									"arguments": `{"cmd":"ls -la"}`,
								},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	resp, err := client.Chat(context.Background(), llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "user", Content: "list files"},
		},
		Tools: []llm.ToolDef{
			{
				Name:        "shell",
				Description: "Execute shell commands",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{"cmd": map[string]any{"type": "string"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "shell" {
		t.Errorf("expected tool name 'shell', got %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].ID != "call_abc123" {
		t.Errorf("expected tool call ID 'call_abc123', got %q", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[0].Arguments != `{"cmd":"ls -la"}` {
		t.Errorf("unexpected arguments: %q", resp.ToolCalls[0].Arguments)
	}
}

func TestClient_Chat_ToolResultMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		// Verify tool result message was sent correctly
		msgs := body["messages"].([]any)
		lastMsg := msgs[len(msgs)-1].(map[string]any)
		if lastMsg["role"] != "tool" {
			t.Errorf("expected last message role 'tool', got %q", lastMsg["role"])
		}
		if lastMsg["tool_call_id"] != "call_abc123" {
			t.Errorf("expected tool_call_id 'call_abc123', got %v", lastMsg["tool_call_id"])
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "Here are the files."}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	resp, err := client.Chat(context.Background(), llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "user", Content: "list files"},
			{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call_abc123", Name: "shell", Arguments: `{"cmd":"ls"}`}}},
			{Role: "tool", ToolCallID: "call_abc123", Content: "file1.txt\nfile2.txt"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Here are the files." {
		t.Errorf("unexpected content: %q", resp.Content)
	}
}
