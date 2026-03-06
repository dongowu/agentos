package orchestration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dongowu/agentos/internal/memory"
)

// MemoryHook bridges the orchestration layer with the memory subsystem.
// It stores task results and recalls relevant context during planning.
// All methods are safe to call with a nil provider -- they return nil/empty.
type MemoryHook struct {
	provider memory.Provider
}

// NewMemoryHook creates a MemoryHook. A nil provider is allowed; all
// operations become no-ops.
func NewMemoryHook(p memory.Provider) *MemoryHook {
	return &MemoryHook{provider: p}
}

// StoreResult marshals the result map to JSON and stores it under
// the key "task:<taskID>".
func (h *MemoryHook) StoreResult(ctx context.Context, taskID string, result map[string]any) error {
	if h.provider == nil {
		return nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("memory_hook: marshal result: %w", err)
	}

	key := "task:" + taskID
	if err := h.provider.Put(ctx, key, data); err != nil {
		return fmt.Errorf("memory_hook: store result: %w", err)
	}
	return nil
}

// RecallContext searches memory for entries relevant to the query and
// returns at most k results.
func (h *MemoryHook) RecallContext(ctx context.Context, query string, k int) ([]memory.SearchResult, error) {
	if h.provider == nil {
		return nil, nil
	}

	results, err := h.provider.Search(ctx, query, k)
	if err != nil {
		return nil, fmt.Errorf("memory_hook: recall context: %w", err)
	}
	return results, nil
}
