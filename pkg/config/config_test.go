package config

import "testing"

func TestApplyEnvOverrides_LLMSettings(t *testing.T) {
	t.Setenv("AGENTOS_LLM_API_KEY", "sk-test")
	t.Setenv("AGENTOS_LLM_BASE_URL", "https://llm.example.com")
	t.Setenv("AGENTOS_LLM_MODEL", "gpt-test")
	t.Setenv("AGENTOS_LLM_PROVIDER", "openai")

	cfg := ApplyEnvOverrides(Config{})
	if cfg.LLM.APIKey != "sk-test" {
		t.Fatalf("expected API key override, got %q", cfg.LLM.APIKey)
	}
	if cfg.LLM.BaseURL != "https://llm.example.com" {
		t.Fatalf("expected base url override, got %q", cfg.LLM.BaseURL)
	}
	if cfg.LLM.Model != "gpt-test" {
		t.Fatalf("expected model override, got %q", cfg.LLM.Model)
	}
	if cfg.LLM.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", cfg.LLM.Provider)
	}
}

func TestApplyEnvOverrides_AutoEnablesOpenAIWhenAPIKeyPresent(t *testing.T) {
	t.Setenv("AGENTOS_LLM_API_KEY", "sk-test")
	t.Setenv("AGENTOS_LLM_PROVIDER", "")

	cfg := ApplyEnvOverrides(Config{LLM: LLMConfig{Provider: "stub"}})
	if cfg.LLM.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", cfg.LLM.Provider)
	}
}
