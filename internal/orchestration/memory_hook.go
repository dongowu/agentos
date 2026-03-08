package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	return h.storeJSON(ctx, "task:"+taskID, result)
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

// StoreScopedResult stores a result payload together with tenant/agent scope metadata.
func (h *MemoryHook) StoreScopedResult(ctx context.Context, tenantID, agentName, prompt, taskID string, result map[string]any) error {
	if tenantID == "" && agentName == "" {
		return h.StoreResult(ctx, taskID, result)
	}
	if result == nil {
		result = map[string]any{}
	}
	scoped := make(map[string]any, len(result)+4)
	for key, value := range result {
		scoped[key] = value
	}
	scoped["tenant_id"] = tenantID
	scoped["agent_name"] = agentName
	scoped["prompt"] = prompt
	scoped["scope"] = memoryScopeKey(tenantID, agentName)
	return h.storeJSON(ctx, memoryScopeKey(tenantID, agentName)+"/task:"+taskID, scoped)
}

// RecallScopedContext limits recall to the caller's tenant/agent scope.
func (h *MemoryHook) RecallScopedContext(ctx context.Context, tenantID, agentName, query string, k int) ([]memory.SearchResult, error) {
	if tenantID == "" && agentName == "" {
		return h.RecallContext(ctx, query, k)
	}
	if h.provider == nil {
		return nil, nil
	}
	scope := strings.ToLower(memoryScopeKey(tenantID, agentName))
	results, err := h.provider.Search(ctx, scope, maxScopedSearch(k))
	if err != nil {
		return nil, fmt.Errorf("memory_hook: recall scoped context: %w", err)
	}
	filtered := make([]memory.SearchResult, 0, len(results))
	for _, result := range results {
		if strings.Contains(strings.ToLower(string(result.Content)), scope) {
			filtered = append(filtered, result)
		}
		if len(filtered) == k {
			break
		}
	}
	return filtered, nil
}

func (h *MemoryHook) storeJSON(ctx context.Context, key string, payload map[string]any) error {
	if h.provider == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("memory_hook: marshal result: %w", err)
	}
	if err := h.provider.Put(ctx, key, data); err != nil {
		return fmt.Errorf("memory_hook: store result: %w", err)
	}
	return nil
}

func memoryScopeKey(tenantID, agentName string) string {
	return fmt.Sprintf("scope:tenant=%s;agent=%s", tenantID, agentName)
}

func maxScopedSearch(k int) int {
	if k < 1 {
		return 4
	}
	return k * 4
}
