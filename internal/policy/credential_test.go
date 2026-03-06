package policy

import (
	"context"
	"testing"
)

func TestInMemoryVault_GetTokenAndResolve(t *testing.T) {
	secrets := map[string]string{
		"worker-1": "sk-real-api-key-1234",
		"worker-2": "sk-real-api-key-5678",
	}
	vault := NewInMemoryVault(secrets)
	ctx := context.Background()

	// Get token for worker-1
	token, err := vault.GetToken(ctx, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	// Token must NOT be the real secret
	if token == "sk-real-api-key-1234" {
		t.Fatal("token must not be the real secret")
	}

	// Resolve token back to real secret
	secret, err := vault.ResolveSecret(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if secret != "sk-real-api-key-1234" {
		t.Fatalf("expected real secret, got %q", secret)
	}
}

func TestInMemoryVault_DifferentAgentsDifferentTokens(t *testing.T) {
	vault := NewInMemoryVault(map[string]string{
		"worker-1": "secret-1",
		"worker-2": "secret-2",
	})
	ctx := context.Background()

	t1, _ := vault.GetToken(ctx, "worker-1")
	t2, _ := vault.GetToken(ctx, "worker-2")

	if t1 == t2 {
		t.Fatal("different agents must receive different tokens")
	}
}

func TestInMemoryVault_StableToken(t *testing.T) {
	vault := NewInMemoryVault(map[string]string{
		"worker-1": "secret-1",
	})
	ctx := context.Background()

	t1, _ := vault.GetToken(ctx, "worker-1")
	t2, _ := vault.GetToken(ctx, "worker-1")

	if t1 != t2 {
		t.Fatal("same agent must always receive the same token")
	}
}

func TestInMemoryVault_UnknownAgent(t *testing.T) {
	vault := NewInMemoryVault(map[string]string{})
	_, err := vault.GetToken(context.Background(), "unknown")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestInMemoryVault_UnknownToken(t *testing.T) {
	vault := NewInMemoryVault(map[string]string{
		"worker-1": "secret-1",
	})
	_, err := vault.ResolveSecret(context.Background(), "bogus-token")
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
}

func TestInMemoryVault_TokenIsOpaque(t *testing.T) {
	vault := NewInMemoryVault(map[string]string{
		"worker-1": "my-super-secret",
	})
	token, _ := vault.GetToken(context.Background(), "worker-1")

	// Token should be a hex string, not contain the secret or agent name
	if token == "my-super-secret" {
		t.Fatal("token must not equal the real secret")
	}
	if token == "worker-1" {
		t.Fatal("token must not equal the agent name")
	}
	if len(token) != 32 { // 16 bytes = 32 hex chars
		t.Fatalf("expected 32-char hex token, got %d chars: %q", len(token), token)
	}
}
