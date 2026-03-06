package config

import "os"

// Config holds adapter and runtime configuration.
type Config struct {
	Messaging   MessagingConfig   `yaml:"messaging"`
	Persistence PersistenceConfig `yaml:"persistence"`
	Runtime     RuntimeConfig     `yaml:"runtime"`
	LLM         LLMConfig         `yaml:"llm"`
	Memory      MemoryConfig      `yaml:"memory"`
	Policy      PolicyConfig      `yaml:"policy"`
	Scheduler   SchedulerConfig   `yaml:"scheduler"`
	AgentDir    string            `yaml:"agent_dir"`
}

// SchedulerConfig configures the task scheduler.
type SchedulerConfig struct {
	Mode               string `yaml:"mode"`                 // local, nats (default: local)
	HeartbeatTimeout   string `yaml:"heartbeat_timeout"`    // e.g. 30s
	HealthCheckInterval string `yaml:"health_check_interval"` // e.g. 10s
}

// PolicyConfig configures the control-plane policy engine.
type PolicyConfig struct {
	DefaultAutonomy string            `yaml:"default_autonomy"` // supervised, semi_autonomous, autonomous
	RateLimit       int               `yaml:"rate_limit"`       // max actions per agent per hour, 0=unlimited
	Rules           []PolicyRuleConfig `yaml:"rules"`
}

// PolicyRuleConfig is a single policy rule from config.
type PolicyRuleConfig struct {
	Agent string   `yaml:"agent"` // glob pattern
	Allow []string `yaml:"allow"` // tool glob patterns
	Deny  []string `yaml:"deny"`  // tool glob patterns
}

// LLMConfig selects and configures the LLM provider for planning.
type LLMConfig struct {
	Provider string `yaml:"provider"` // stub, openai (default: stub)
	Model    string `yaml:"model"`    // e.g. gpt-4o, gemini-3.5-pro
	BaseURL  string `yaml:"base_url"` // e.g. https://api.openai.com
	APIKey   string `yaml:"api_key"`
}

// MemoryConfig selects and configures the agent memory backend.
type MemoryConfig struct {
	Provider string `yaml:"provider"` // inmemory, redis (default: inmemory)
	TTL      string `yaml:"ttl"`      // e.g. 24h, 1h30m
	Redis    RedisMemoryConfig `yaml:"redis"`
}

// RedisMemoryConfig is used when memory provider=redis.
type RedisMemoryConfig struct {
	Addr   string `yaml:"addr"`   // 127.0.0.1:6379
	Prefix string `yaml:"prefix"` // key prefix
}

// RuntimeConfig configures the execution layer.
type RuntimeConfig struct {
	WorkerAddr string `yaml:"worker_addr"` // localhost:50051 for gRPC worker
}

// MessagingConfig selects and configures the event bus adapter.
type MessagingConfig struct {
	Provider string      `yaml:"provider"` // memory, nats (default: nats)
	NATS     NATSConfig  `yaml:"nats"`
}

// NATSConfig is used when provider=nats.
type NATSConfig struct {
	URL    string `yaml:"url"`    // nats://localhost:4222
	Stream string `yaml:"stream"` // AGENTOS
}

// PersistenceConfig selects and configures the task store adapter.
type PersistenceConfig struct {
	Provider  string         `yaml:"provider"`  // memory, postgres (default: postgres)
	Postgres  PostgresConfig `yaml:"postgres"`
}

// PostgresConfig is used when provider=postgres.
type PostgresConfig struct {
	DSN string `yaml:"dsn"` // postgres://user:pass@localhost:5432/agentos?sslmode=disable
}

// Default returns production defaults (NATS + Postgres + OpenAI LLM).
func Default() Config {
	return Config{
		Runtime: RuntimeConfig{WorkerAddr: "localhost:50051"},
		Messaging: MessagingConfig{
			Provider: "nats",
			NATS: NATSConfig{
				URL:    "nats://localhost:4222",
				Stream: "AGENTOS",
			},
		},
		Persistence: PersistenceConfig{
			Provider: "postgres",
			Postgres: PostgresConfig{
				DSN: "postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable",
			},
		},
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o",
			BaseURL:  "https://api.openai.com",
		},
		Memory: MemoryConfig{
			Provider: "inmemory",
			TTL:      "24h",
		},
		AgentDir: "agents",
		Policy: PolicyConfig{
			DefaultAutonomy: "supervised",
		},
		Scheduler: SchedulerConfig{
			Mode:               "local",
			HeartbeatTimeout:   "30s",
			HealthCheckInterval: "10s",
		},
	}
}

// Dev returns in-memory adapters for local development.
// Set AGENTOS_WORKER_ADDR to enable execution (e.g. localhost:50051).
// Set AGENTOS_LLM_API_KEY to enable LLM planning.
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
	return Config{
		Runtime:     RuntimeConfig{WorkerAddr: addr},
		Messaging:   MessagingConfig{Provider: "memory"},
		Persistence: PersistenceConfig{Provider: "memory"},
		LLM: LLMConfig{
			Provider: llmProvider,
			Model:    model,
			BaseURL:  baseURL,
			APIKey:   apiKey,
		},
		Memory: MemoryConfig{
			Provider: "inmemory",
		},
		Policy: PolicyConfig{
			DefaultAutonomy: "autonomous",
		},
		Scheduler: SchedulerConfig{
			Mode: "local",
		},
	}
}
