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
