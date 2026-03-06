package redis

import (
	"context"
	"testing"

	"github.com/dongowu/agentos/internal/memory"
)

// Compile-time interface check.
var _ memory.Provider = (*Redis)(nil)

func TestPutAndGet(t *testing.T) {
	r, cleanup := newTestRedis(t)
	defer cleanup()
	ctx := context.Background()

	if err := r.Put(ctx, "k1", []byte("v1")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := r.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v1" {
		t.Fatalf("Get = %q, want %q", got, "v1")
	}
}

func TestGetMissing(t *testing.T) {
	r, cleanup := newTestRedis(t)
	defer cleanup()
	ctx := context.Background()

	_, err := r.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}

func TestSearch(t *testing.T) {
	r, cleanup := newTestRedis(t)
	defer cleanup()
	ctx := context.Background()

	r.Put(ctx, "agentos:test:greeting", []byte("hello world"))
	r.Put(ctx, "agentos:test:farewell", []byte("goodbye world"))
	r.Put(ctx, "agentos:test:other", []byte("foo bar"))

	results, err := r.Search(ctx, "world", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search returned %d results, want 2", len(results))
	}
}

// newTestRedis returns a Redis provider backed by a fake server for unit testing,
// or skips the test if AGENTOS_REDIS_ADDR is set (integration mode not available).
func newTestRedis(t *testing.T) (*Redis, func()) {
	t.Helper()

	srv := newFakeServer(t)
	r, err := New(srv.Addr(), WithPrefix("agentos:test:"))
	if err != nil {
		srv.Close()
		t.Fatalf("New: %v", err)
	}
	return r, func() {
		r.Close()
		srv.Close()
	}
}
