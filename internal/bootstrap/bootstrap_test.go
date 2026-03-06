package bootstrap

import (
	"testing"

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
