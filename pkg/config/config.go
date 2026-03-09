package config

import (
	"os"
	"strings"
)

// Config holds adapter and runtime configuration.
type Config struct {
	Messaging   MessagingConfig   `yaml:"messaging"`
	Persistence PersistenceConfig `yaml:"persistence"`
	Runtime     RuntimeConfig     `yaml:"runtime"`
	LLM         LLMConfig         `yaml:"llm"`
	Memory      MemoryConfig      `yaml:"memory"`
	Policy      PolicyConfig      `yaml:"policy"`
	Scheduler   SchedulerConfig   `yaml:"scheduler"`
	Vault       VaultConfig       `yaml:"vault"`
	Auth        AuthConfig        `yaml:"auth"`
	AgentDir    string            `yaml:"agent_dir"`
}

// SchedulerConfig configures the task scheduler.
type SchedulerConfig struct {
	Mode                string `yaml:"mode"`
	HeartbeatTimeout    string `yaml:"heartbeat_timeout"`
	HealthCheckInterval string `yaml:"health_check_interval"`
	ControlPlaneAddr    string `yaml:"control_plane_addr"`
}

// PolicyConfig configures the control-plane policy engine.
type PolicyConfig struct {
	DefaultAutonomy string             `yaml:"default_autonomy"`
	RateLimit       int                `yaml:"rate_limit"`
	Rules           []PolicyRuleConfig `yaml:"rules"`
}

// PolicyRuleConfig is a single policy rule from config.
type PolicyRuleConfig struct {
	Agent            string   `yaml:"agent"`
	Allow            []string `yaml:"allow"`
	Deny             []string `yaml:"deny"`
	ApprovalRequired []string `yaml:"approval_required"`
}

// AuthConfig configures caller authentication for HTTP task APIs.
type AuthConfig struct {
	Tokens map[string]AuthPrincipalConfig `yaml:"tokens"`
}

// AuthPrincipalConfig binds a bearer token to a subject and tenant.
type AuthPrincipalConfig struct {
	Subject  string   `yaml:"subject"`
	TenantID string   `yaml:"tenant_id"`
	Roles    []string `yaml:"roles"`
}

// VaultConfig configures opaque agent credential tokens.
type VaultConfig struct {
	AgentSecrets map[string]string `yaml:"agent_secrets"`
}

// LLMConfig selects and configures the LLM provider for planning.
type LLMConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"`
}

// MemoryConfig selects and configures the agent memory backend.
type MemoryConfig struct {
	Provider string            `yaml:"provider"`
	TTL      string            `yaml:"ttl"`
	Redis    RedisMemoryConfig `yaml:"redis"`
}

// RedisMemoryConfig is used when memory provider=redis.
type RedisMemoryConfig struct {
	Addr   string `yaml:"addr"`
	Prefix string `yaml:"prefix"`
}

// RuntimeConfig configures the execution layer.
type RuntimeConfig struct {
	WorkerAddr string `yaml:"worker_addr"`
}

// MessagingConfig selects and configures the event bus adapter.
type MessagingConfig struct {
	Provider string     `yaml:"provider"`
	NATS     NATSConfig `yaml:"nats"`
}

// NATSConfig is used when provider=nats.
type NATSConfig struct {
	URL    string `yaml:"url"`
	Stream string `yaml:"stream"`
}

// PersistenceConfig selects and configures the task store adapter.
type PersistenceConfig struct {
	Provider string         `yaml:"provider"`
	Postgres PostgresConfig `yaml:"postgres"`
}

// PostgresConfig is used when provider=postgres.
type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

// Default returns production defaults.
func Default() Config {
	return Config{
		Runtime: RuntimeConfig{WorkerAddr: "localhost:50051"},
		Messaging: MessagingConfig{
			Provider: "nats",
			NATS:     NATSConfig{URL: "nats://localhost:4222", Stream: "AGENTOS"},
		},
		Persistence: PersistenceConfig{
			Provider: "postgres",
			Postgres: PostgresConfig{DSN: "postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable"},
		},
		LLM:       LLMConfig{Provider: "openai", Model: "gpt-4o", BaseURL: "https://api.openai.com"},
		Memory:    MemoryConfig{Provider: "inmemory", TTL: "24h"},
		AgentDir:  "agents",
		Policy:    PolicyConfig{DefaultAutonomy: "supervised"},
		Scheduler: SchedulerConfig{Mode: "nats", HeartbeatTimeout: "30s", HealthCheckInterval: "10s"},
		Vault:     VaultConfig{AgentSecrets: map[string]string{}},
		Auth:      AuthConfig{Tokens: map[string]AuthPrincipalConfig{}},
	}
}

// Dev returns in-memory adapters for local development.
func Dev() Config {
	addr := os.Getenv("AGENTOS_WORKER_ADDR")
	apiKey := os.Getenv("AGENTOS_LLM_API_KEY")
	baseURL := os.Getenv("AGENTOS_LLM_BASE_URL")
	model := os.Getenv("AGENTOS_LLM_MODEL")
	llmProvider := "stub"
	if apiKey != "" {
		llmProvider = "openai"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if model == "" {
		model = "gpt-4o"
	}
	return ApplyEnvOverrides(Config{
		Runtime:     RuntimeConfig{WorkerAddr: addr},
		Messaging:   MessagingConfig{Provider: "memory"},
		Persistence: PersistenceConfig{Provider: "memory"},
		LLM:         LLMConfig{Provider: llmProvider, Model: model, BaseURL: baseURL, APIKey: apiKey},
		Memory:      MemoryConfig{Provider: "inmemory"},
		Policy:      PolicyConfig{DefaultAutonomy: "autonomous"},
		Scheduler:   SchedulerConfig{Mode: "local", HeartbeatTimeout: "30s", HealthCheckInterval: "10s"},
		Vault:       VaultConfig{AgentSecrets: map[string]string{}},
		Auth:        AuthConfig{Tokens: map[string]AuthPrincipalConfig{}},
	})
}

// ApplyEnvOverrides overlays well-known runtime environment variables onto cfg.
func ApplyEnvOverrides(cfg Config) Config {
	if addr := os.Getenv("AGENTOS_WORKER_ADDR"); addr != "" {
		cfg.Runtime.WorkerAddr = addr
	}
	if controlPlane := os.Getenv("AGENTOS_CONTROL_PLANE_ADDR"); controlPlane != "" {
		cfg.Scheduler.ControlPlaneAddr = controlPlane
	}
	if mode := os.Getenv("AGENTOS_SCHEDULER_MODE"); mode != "" {
		cfg.Scheduler.Mode = mode
	}
	if natsURL := os.Getenv("AGENTOS_NATS_URL"); natsURL != "" {
		cfg.Messaging.NATS.URL = natsURL
	}
	if natsStream := os.Getenv("AGENTOS_NATS_STREAM"); natsStream != "" {
		cfg.Messaging.NATS.Stream = natsStream
	}
	if postgresDSN := os.Getenv("AGENTOS_POSTGRES_DSN"); postgresDSN != "" {
		cfg.Persistence.Postgres.DSN = postgresDSN
	}
	if provider := os.Getenv("AGENTOS_LLM_PROVIDER"); provider != "" {
		cfg.LLM.Provider = provider
	}
	if apiKey := os.Getenv("AGENTOS_LLM_API_KEY"); apiKey != "" {
		cfg.LLM.APIKey = apiKey
	}
	if baseURL := os.Getenv("AGENTOS_LLM_BASE_URL"); baseURL != "" {
		cfg.LLM.BaseURL = baseURL
	}
	if model := os.Getenv("AGENTOS_LLM_MODEL"); model != "" {
		cfg.LLM.Model = model
	}
	if cfg.LLM.APIKey != "" && (cfg.LLM.Provider == "" || cfg.LLM.Provider == "stub" || cfg.LLM.Provider == "auto") {
		cfg.LLM.Provider = "openai"
	}
	if rawSecrets := os.Getenv("AGENTOS_AGENT_SECRETS"); rawSecrets != "" {
		cfg.Vault.AgentSecrets = parseAgentSecrets(rawSecrets)
	}
	if rawTokens := os.Getenv("AGENTOS_AUTH_TOKENS"); rawTokens != "" {
		cfg.Auth.Tokens = parseAuthTokens(rawTokens)
	}
	if rawApprovalRequired := os.Getenv("AGENTOS_POLICY_APPROVAL_REQUIRED"); rawApprovalRequired != "" {
		cfg.Policy.Rules = append(cfg.Policy.Rules, PolicyRuleConfig{
			Agent:            "*",
			ApprovalRequired: parseList(rawApprovalRequired),
		})
	}
	return cfg
}

func parseList(raw string) []string {
	items := make([]string, 0)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		items = append(items, entry)
	}
	return items
}

func parseAgentSecrets(raw string) map[string]string {
	secrets := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		agent, secret, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		agent = strings.TrimSpace(agent)
		secret = strings.TrimSpace(secret)
		if agent == "" || secret == "" {
			continue
		}
		secrets[agent] = secret
	}
	return secrets
}

func parseAuthTokens(raw string) map[string]AuthPrincipalConfig {
	tokens := map[string]AuthPrincipalConfig{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		token, payload, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		token = strings.TrimSpace(token)
		payload = strings.TrimSpace(payload)
		if token == "" || payload == "" {
			continue
		}
		parts := strings.SplitN(payload, "|", 3)
		principal := AuthPrincipalConfig{}
		principal.Subject = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			principal.TenantID = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			for _, role := range strings.Split(parts[2], ";") {
				role = strings.TrimSpace(role)
				if role != "" {
					principal.Roles = append(principal.Roles, role)
				}
			}
		}
		if principal.Subject == "" {
			continue
		}
		tokens[token] = principal
	}
	return tokens
}
