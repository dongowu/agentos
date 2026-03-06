// Package memory provides short-term and long-term memory for ClawOS agents.
package memory

import "context"

// Provider is the memory interface.
// Short-term: Redis. Long-term: Vector DB.
type Provider interface {
	Put(ctx context.Context, key string, value []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Search(ctx context.Context, query string, k int) ([]SearchResult, error)
}

// SearchResult is a single result from vector search.
type SearchResult struct {
	Key     string
	Content []byte
	Score   float64
}
