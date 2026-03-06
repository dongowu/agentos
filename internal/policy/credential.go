package policy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// CredentialVault provides credential isolation following HiClaw's pattern:
// workers receive opaque tokens, real secrets are resolved only in the
// control plane (gateway).
type CredentialVault interface {
	// GetToken returns an opaque consumer token for the given agent.
	// The token can be safely passed to workers.
	GetToken(ctx context.Context, agentName string) (string, error)

	// ResolveSecret maps an opaque token back to the real credential.
	// This must only be called from the gateway/control plane.
	ResolveSecret(ctx context.Context, token string) (string, error)
}

// InMemoryVault is a CredentialVault backed by in-memory maps.
// Suitable for development and testing.
type InMemoryVault struct {
	mu            sync.RWMutex
	agentTokens   map[string]string // agentName -> token
	tokenSecrets  map[string]string // token -> real secret
}

// NewInMemoryVault creates a vault pre-loaded with agent-to-secret mappings.
func NewInMemoryVault(secrets map[string]string) *InMemoryVault {
	v := &InMemoryVault{
		agentTokens:  make(map[string]string, len(secrets)),
		tokenSecrets: make(map[string]string, len(secrets)),
	}
	for agent, secret := range secrets {
		token := generateToken()
		v.agentTokens[agent] = token
		v.tokenSecrets[token] = secret
	}
	return v
}

// GetToken returns the opaque token for the named agent.
func (v *InMemoryVault) GetToken(_ context.Context, agentName string) (string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	token, ok := v.agentTokens[agentName]
	if !ok {
		return "", fmt.Errorf("no credential registered for agent %q", agentName)
	}
	return token, nil
}

// ResolveSecret maps an opaque token to the real secret.
func (v *InMemoryVault) ResolveSecret(_ context.Context, token string) (string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	secret, ok := v.tokenSecrets[token]
	if !ok {
		return "", fmt.Errorf("unknown token")
	}
	return secret, nil
}

func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
