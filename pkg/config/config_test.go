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

func TestApplyEnvOverrides_SchedulerMode(t *testing.T) {
	t.Setenv("AGENTOS_SCHEDULER_MODE", "nats")

	cfg := ApplyEnvOverrides(Config{Scheduler: SchedulerConfig{Mode: "local"}})
	if cfg.Scheduler.Mode != "nats" {
		t.Fatalf("expected scheduler mode nats, got %q", cfg.Scheduler.Mode)
	}
}

func TestApplyEnvOverrides_SchedulerRetryPolicy(t *testing.T) {
	t.Setenv("AGENTOS_SCHEDULER_SUBMIT_RETRIES", "3")
	t.Setenv("AGENTOS_SCHEDULER_SUBMIT_RETRY_BACKOFF", "125ms")

	cfg := ApplyEnvOverrides(Config{Scheduler: SchedulerConfig{SubmitRetries: 1, SubmitRetryBackoff: "25ms"}})
	if cfg.Scheduler.SubmitRetries != 3 {
		t.Fatalf("expected submit retries override, got %d", cfg.Scheduler.SubmitRetries)
	}
	if cfg.Scheduler.SubmitRetryBackoff != "125ms" {
		t.Fatalf("expected submit retry backoff override, got %q", cfg.Scheduler.SubmitRetryBackoff)
	}
}

func TestApplyEnvOverrides_SchedulerHealthMonitoring(t *testing.T) {
	t.Setenv("AGENTOS_SCHEDULER_HEARTBEAT_TIMEOUT", "3s")
	t.Setenv("AGENTOS_SCHEDULER_HEALTH_CHECK_INTERVAL", "250ms")

	cfg := ApplyEnvOverrides(Config{
		Scheduler: SchedulerConfig{
			HeartbeatTimeout:    "30s",
			HealthCheckInterval: "10s",
		},
	})
	if cfg.Scheduler.HeartbeatTimeout != "3s" {
		t.Fatalf("expected heartbeat timeout override, got %q", cfg.Scheduler.HeartbeatTimeout)
	}
	if cfg.Scheduler.HealthCheckInterval != "250ms" {
		t.Fatalf("expected health check interval override, got %q", cfg.Scheduler.HealthCheckInterval)
	}
}

func TestApplyEnvOverrides_SchedulerRecovery(t *testing.T) {
	t.Setenv("AGENTOS_SCHEDULER_RECOVERY_ENABLED", "false")
	t.Setenv("AGENTOS_SCHEDULER_STALE_RUNNING_TIMEOUT", "45m")

	cfg := ApplyEnvOverrides(Config{Scheduler: SchedulerConfig{RecoveryEnabled: true, StaleRunningTimeout: "15m"}})
	if cfg.Scheduler.RecoveryEnabled {
		t.Fatal("expected recovery enabled override to be false")
	}
	if cfg.Scheduler.StaleRunningTimeout != "45m" {
		t.Fatalf("expected stale running timeout override, got %q", cfg.Scheduler.StaleRunningTimeout)
	}
}

func TestApplyEnvOverrides_AuthTokens(t *testing.T) {
	t.Setenv("AGENTOS_AUTH_TOKENS", "token-a=user-a|tenant-a|admin;writer,token-b=user-b|tenant-b")

	cfg := ApplyEnvOverrides(Config{})
	if len(cfg.Auth.Tokens) != 2 {
		t.Fatalf("expected 2 auth tokens, got %d", len(cfg.Auth.Tokens))
	}
	if cfg.Auth.Tokens["token-a"].Subject != "user-a" {
		t.Fatalf("expected subject user-a, got %q", cfg.Auth.Tokens["token-a"].Subject)
	}
	if cfg.Auth.Tokens["token-a"].TenantID != "tenant-a" {
		t.Fatalf("expected tenant tenant-a, got %q", cfg.Auth.Tokens["token-a"].TenantID)
	}
	if len(cfg.Auth.Tokens["token-a"].Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(cfg.Auth.Tokens["token-a"].Roles))
	}
	if cfg.Auth.Tokens["token-b"].TenantID != "tenant-b" {
		t.Fatalf("expected tenant tenant-b, got %q", cfg.Auth.Tokens["token-b"].TenantID)
	}
}

func TestApplyEnvOverrides_MessagingAndPersistence(t *testing.T) {
	t.Setenv("AGENTOS_NATS_URL", "nats://nats.example.com:4222")
	t.Setenv("AGENTOS_NATS_STREAM", "AGENTOS_TEST")
	t.Setenv("AGENTOS_POSTGRES_DSN", "postgres://user:pass@db.example.com:5432/agentos?sslmode=disable")

	cfg := ApplyEnvOverrides(Config{})
	if cfg.Messaging.NATS.URL != "nats://nats.example.com:4222" {
		t.Fatalf("expected nats url override, got %q", cfg.Messaging.NATS.URL)
	}
	if cfg.Messaging.NATS.Stream != "AGENTOS_TEST" {
		t.Fatalf("expected nats stream override, got %q", cfg.Messaging.NATS.Stream)
	}
	if cfg.Persistence.Postgres.DSN != "postgres://user:pass@db.example.com:5432/agentos?sslmode=disable" {
		t.Fatalf("expected postgres dsn override, got %q", cfg.Persistence.Postgres.DSN)
	}
}
