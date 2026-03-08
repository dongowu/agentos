package bootstrap

import (
	"context"
	"testing"

	"github.com/dongowu/agentos/internal/adapter"
	"github.com/dongowu/agentos/internal/adapters/llm"
	"github.com/dongowu/agentos/internal/orchestration"
	"github.com/dongowu/agentos/pkg/config"
)

func TestPlannerFromConfig_FallsBackWithoutAPIKey(t *testing.T) {
	planner := plannerFromConfig(config.Config{LLM: config.LLMConfig{Provider: "openai", Model: "gpt-4o"}})
	if _, ok := planner.(*orchestration.PromptPlanner); !ok {
		t.Fatalf("expected PromptPlanner fallback, got %T", planner)
	}
}

func TestPlannerFromConfig_UsesFallbackPipelineWithAPIKey(t *testing.T) {
	planner := plannerFromConfig(config.Config{LLM: config.LLMConfig{Provider: "openai", Model: "gpt-4o", BaseURL: "https://api.openai.com", APIKey: "sk-test"}})
	if _, ok := planner.(*orchestration.FallbackPlanner); !ok {
		t.Fatalf("expected FallbackPlanner, got %T", planner)
	}
}

func TestPlannerFromConfig_UnsupportedProviderFallsBackToPrompt(t *testing.T) {
	planner := plannerFromConfig(config.Config{LLM: config.LLMConfig{Provider: "anthropic", APIKey: "secret"}})
	if _, ok := planner.(*orchestration.PromptPlanner); !ok {
		t.Fatalf("expected PromptPlanner for unsupported provider, got %T", planner)
	}
}

func TestAuthProviderFromConfig_UsesStaticBearerTokens(t *testing.T) {
	provider := authProviderFromConfig(config.Config{Auth: config.AuthConfig{Tokens: map[string]config.AuthPrincipalConfig{
		"token-a": {Subject: "user-a", TenantID: "tenant-a", Roles: []string{"admin"}},
	}}})
	if provider == nil {
		t.Fatal("expected auth provider, got nil")
	}
}

type bootstrapMockProvider struct{}

func (bootstrapMockProvider) Chat(_ context.Context, _ llm.Request) (*llm.Response, error) {
	return &llm.Response{Content: `{"Actions":[{"Kind":"command.exec","Payload":{"cmd":"echo ok"}}]}`}, nil
}

func TestPlannerFromConfig_UsesRegisteredProvider(t *testing.T) {
	providerName := "bootstrap-test-provider"
	adapter.RegisterLLMProvider(providerName, func(cfg config.LLMConfig) (llm.Provider, string, error) {
		return bootstrapMockProvider{}, cfg.Model, nil
	})
	planner := plannerFromConfig(config.Config{LLM: config.LLMConfig{Provider: providerName, Model: "custom-model", APIKey: "unused"}})
	if _, ok := planner.(*orchestration.FallbackPlanner); !ok {
		t.Fatalf("expected FallbackPlanner, got %T", planner)
	}
}
