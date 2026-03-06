// Package inmemory provides a thread-safe in-memory implementation of memory.Provider.
package inmemory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dongowu/agentos/internal/memory"
)

// InMemory is a thread-safe in-memory key-value store implementing memory.Provider.
type InMemory struct {
	mu    sync.RWMutex
	store map[string]entry
	ttl   time.Duration
}

type entry struct {
	value   []byte
	expires time.Time // zero means no expiry
}

// Option configures an InMemory provider.
type Option func(*InMemory)

// WithTTL sets a default time-to-live for all entries.
func WithTTL(d time.Duration) Option {
	return func(m *InMemory) {
		m.ttl = d
	}
}

// New creates a new in-memory provider.
func New(opts ...Option) *InMemory {
	m := &InMemory{
		store: make(map[string]entry),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Put stores a value under the given key.
func (m *InMemory) Put(_ context.Context, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	e := entry{value: make([]byte, len(value))}
	copy(e.value, value)

	if m.ttl > 0 {
		e.expires = time.Now().Add(m.ttl)
	}

	m.store[key] = e
	return nil
}

// Get retrieves a value by key. Returns an error if the key does not exist or has expired.
func (m *InMemory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	e, ok := m.store[key]
	m.mu.RUnlock()

	if !ok {
		return nil, errors.New("key not found: " + key)
	}
	if !e.expires.IsZero() && time.Now().After(e.expires) {
		m.mu.Lock()
		delete(m.store, key)
		m.mu.Unlock()
		return nil, errors.New("key expired: " + key)
	}
	return e.value, nil
}

// Search returns entries whose content contains the query substring (case-insensitive).
// Results are scored by the ratio of query length to content length and capped at k.
func (m *InMemory) Search(_ context.Context, query string, k int) ([]memory.SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryLower := strings.ToLower(query)
	now := time.Now()
	var results []memory.SearchResult

	for key, e := range m.store {
		if !e.expires.IsZero() && now.After(e.expires) {
			continue
		}
		contentLower := strings.ToLower(string(e.value))
		if strings.Contains(contentLower, queryLower) {
			score := float64(len(query)) / float64(len(e.value))
			if score > 1 {
				score = 1
			}
			results = append(results, memory.SearchResult{
				Key:     key,
				Content: e.value,
				Score:   score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > k {
		results = results[:k]
	}
	return results, nil
}
