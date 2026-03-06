package inmemory

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/memory"
)

// Compile-time interface check.
var _ memory.Provider = (*InMemory)(nil)

func TestPutAndGet(t *testing.T) {
	m := New()
	ctx := context.Background()

	if err := m.Put(ctx, "key1", []byte("value1")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := m.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "value1" {
		t.Fatalf("Get = %q, want %q", got, "value1")
	}
}

func TestGetMissing(t *testing.T) {
	m := New()
	ctx := context.Background()

	_, err := m.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}

func TestPutOverwrite(t *testing.T) {
	m := New()
	ctx := context.Background()

	m.Put(ctx, "k", []byte("v1"))
	m.Put(ctx, "k", []byte("v2"))

	got, err := m.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v2" {
		t.Fatalf("Get = %q, want %q after overwrite", got, "v2")
	}
}

func TestSearch(t *testing.T) {
	m := New()
	ctx := context.Background()

	m.Put(ctx, "greeting", []byte("hello world"))
	m.Put(ctx, "farewell", []byte("goodbye world"))
	m.Put(ctx, "unrelated", []byte("foo bar"))

	results, err := m.Search(ctx, "world", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search returned %d results, want 2", len(results))
	}
	for _, r := range results {
		if !strings.Contains(string(r.Content), "world") {
			t.Errorf("result %q does not contain 'world'", r.Key)
		}
		if r.Score <= 0 || r.Score > 1 {
			t.Errorf("result %q score %f not in (0,1]", r.Key, r.Score)
		}
	}
}

func TestSearchLimit(t *testing.T) {
	m := New()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		m.Put(ctx, string(rune('a'+i)), []byte("match"))
	}

	results, err := m.Search(ctx, "match", 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Search returned %d results, want 3", len(results))
	}
}

func TestSearchNoResults(t *testing.T) {
	m := New()
	ctx := context.Background()

	m.Put(ctx, "k", []byte("hello"))

	results, err := m.Search(ctx, "zzz", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search returned %d results, want 0", len(results))
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	m := New()
	ctx := context.Background()

	m.Put(ctx, "k", []byte("Hello World"))

	results, err := m.Search(ctx, "hello", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search returned %d results, want 1", len(results))
	}
}

func TestTTLExpiry(t *testing.T) {
	m := New(WithTTL(50 * time.Millisecond))
	ctx := context.Background()

	m.Put(ctx, "ephemeral", []byte("data"))

	got, err := m.Get(ctx, "ephemeral")
	if err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}
	if string(got) != "data" {
		t.Fatalf("Get = %q, want %q", got, "data")
	}

	time.Sleep(60 * time.Millisecond)

	_, err = m.Get(ctx, "ephemeral")
	if err == nil {
		t.Fatal("expected error after TTL expiry, got nil")
	}
}

func TestTTLExpiryExcludesFromSearch(t *testing.T) {
	m := New(WithTTL(50 * time.Millisecond))
	ctx := context.Background()

	m.Put(ctx, "temp", []byte("findme"))
	time.Sleep(60 * time.Millisecond)

	results, err := m.Search(ctx, "findme", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search returned %d results after TTL, want 0", len(results))
	}
}

func TestConcurrency(t *testing.T) {
	m := New()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('A' + i%26))
			m.Put(ctx, key, []byte("val"))
			m.Get(ctx, key)
			m.Search(ctx, "val", 5)
		}(i)
	}
	wg.Wait()
}
