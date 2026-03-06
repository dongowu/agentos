package builder

import (
	"context"
	"testing"
	"time"
)

func TestNewProviderInMemory(t *testing.T) {
	p, err := NewProvider("inmemory", nil)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ctx := context.Background()

	if err := p.Put(ctx, "k", []byte("v")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := p.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v" {
		t.Fatalf("Get = %q, want %q", got, "v")
	}
}

func TestNewProviderInMemoryWithTTL(t *testing.T) {
	opts := map[string]any{"ttl": 50 * time.Millisecond}
	p, err := NewProvider("inmemory", opts)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ctx := context.Background()

	p.Put(ctx, "k", []byte("v"))
	time.Sleep(60 * time.Millisecond)
	_, err = p.Get(ctx, "k")
	if err == nil {
		t.Fatal("expected error after TTL expiry")
	}
}

func TestNewProviderInMemoryWithTTLString(t *testing.T) {
	opts := map[string]any{"ttl": "100ms"}
	p, err := NewProvider("inmemory", opts)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}

func TestNewProviderUnknown(t *testing.T) {
	_, err := NewProvider("unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown provider type")
	}
}
