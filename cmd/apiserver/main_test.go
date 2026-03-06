package main

import "testing"

func TestAPIServerListenAddr_Default(t *testing.T) {
	t.Setenv("AGENTOS_API_LISTEN_ADDR", "")
	if got := apiListenAddr(); got != ":8080" {
		t.Fatalf("expected default :8080, got %q", got)
	}
}

func TestAPIServerListenAddr_UsesEnvOverride(t *testing.T) {
	t.Setenv("AGENTOS_API_LISTEN_ADDR", "127.0.0.1:18080")
	if got := apiListenAddr(); got != "127.0.0.1:18080" {
		t.Fatalf("expected overridden addr, got %q", got)
	}
}
