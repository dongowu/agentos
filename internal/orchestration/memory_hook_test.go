package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dongowu/agentos/internal/memory"
)

// --- mock provider --------------------------------------------------------

type mockMemoryProvider struct {
	putCalls    []putCall
	getCalls    []string
	searchCalls []searchCall

	putErr    error
	getResult []byte
	getErr    error
	searchRes []memory.SearchResult
	searchErr error
}

type putCall struct {
	key   string
	value []byte
}

type searchCall struct {
	query string
	k     int
}

func (m *mockMemoryProvider) Put(_ context.Context, key string, value []byte) error {
	m.putCalls = append(m.putCalls, putCall{key, value})
	return m.putErr
}

func (m *mockMemoryProvider) Get(_ context.Context, key string) ([]byte, error) {
	m.getCalls = append(m.getCalls, key)
	return m.getResult, m.getErr
}

func (m *mockMemoryProvider) Search(_ context.Context, query string, k int) ([]memory.SearchResult, error) {
	m.searchCalls = append(m.searchCalls, searchCall{query, k})
	return m.searchRes, m.searchErr
}

// --- tests ----------------------------------------------------------------

func TestNewMemoryHook_NilProvider(t *testing.T) {
	h := NewMemoryHook(nil)
	if h == nil {
		t.Fatal("expected non-nil MemoryHook")
	}
	if h.provider != nil {
		t.Fatal("expected nil provider")
	}
}

func TestStoreResult_NilProvider_NoOp(t *testing.T) {
	h := NewMemoryHook(nil)
	err := h.StoreResult(context.Background(), "task-1", map[string]any{"status": "ok"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestStoreResult_MarshalAndPut(t *testing.T) {
	mock := &mockMemoryProvider{}
	h := NewMemoryHook(mock)

	result := map[string]any{
		"exit_code": 0,
		"output":    "hello world",
	}

	err := h.StoreResult(context.Background(), "abc-123", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.putCalls) != 1 {
		t.Fatalf("expected 1 Put call, got %d", len(mock.putCalls))
	}

	call := mock.putCalls[0]
	if call.key != "task:abc-123" {
		t.Errorf("expected key 'task:abc-123', got %q", call.key)
	}

	var stored map[string]any
	if err := json.Unmarshal(call.value, &stored); err != nil {
		t.Fatalf("stored value is not valid JSON: %v", err)
	}
	if stored["output"] != "hello world" {
		t.Errorf("expected output 'hello world', got %v", stored["output"])
	}
}

func TestStoreResult_ProviderError(t *testing.T) {
	mock := &mockMemoryProvider{putErr: errors.New("storage down")}
	h := NewMemoryHook(mock)

	err := h.StoreResult(context.Background(), "t1", map[string]any{"x": 1})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, mock.putErr) {
		t.Errorf("expected wrapped provider error, got: %v", err)
	}
}

func TestRecallContext_NilProvider_ReturnsNil(t *testing.T) {
	h := NewMemoryHook(nil)
	results, err := h.RecallContext(context.Background(), "deploy", 5)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results, got %v", results)
	}
}

func TestRecallContext_ReturnsSearchResults(t *testing.T) {
	expected := []memory.SearchResult{
		{Key: "task:old-1", Content: []byte("previous deploy"), Score: 0.95},
		{Key: "task:old-2", Content: []byte("another deploy"), Score: 0.80},
	}
	mock := &mockMemoryProvider{searchRes: expected}
	h := NewMemoryHook(mock)

	results, err := h.RecallContext(context.Background(), "deploy project", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.searchCalls) != 1 {
		t.Fatalf("expected 1 Search call, got %d", len(mock.searchCalls))
	}
	if mock.searchCalls[0].query != "deploy project" {
		t.Errorf("expected query 'deploy project', got %q", mock.searchCalls[0].query)
	}
	if mock.searchCalls[0].k != 3 {
		t.Errorf("expected k=3, got %d", mock.searchCalls[0].k)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Key != "task:old-1" {
		t.Errorf("unexpected first result key: %q", results[0].Key)
	}
}

func TestRecallContext_ProviderError(t *testing.T) {
	mock := &mockMemoryProvider{searchErr: errors.New("index unavailable")}
	h := NewMemoryHook(mock)

	_, err := h.RecallContext(context.Background(), "query", 5)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, mock.searchErr) {
		t.Errorf("expected wrapped provider error, got: %v", err)
	}
}
