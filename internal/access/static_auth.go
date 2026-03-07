package access

import (
	"context"
	"errors"
)

var ErrUnauthorized = errors.New("unauthorized")

// StaticBearerAuthProvider authenticates fixed bearer tokens to principals.
type StaticBearerAuthProvider struct {
	tokens map[string]Principal
}

func NewStaticBearerAuthProvider(tokens map[string]Principal) *StaticBearerAuthProvider {
	cloned := make(map[string]Principal, len(tokens))
	for token, principal := range tokens {
		cloned[token] = principal
	}
	return &StaticBearerAuthProvider{tokens: cloned}
}

func (p *StaticBearerAuthProvider) Authenticate(_ context.Context, token string) (*Principal, error) {
	principal, ok := p.tokens[token]
	if !ok {
		return nil, ErrUnauthorized
	}
	copyPrincipal := principal
	copyPrincipal.Roles = append([]string(nil), principal.Roles...)
	return &copyPrincipal, nil
}
