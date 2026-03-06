package config

import "os"

// Config holds adapter and runtime configuration.
type Config struct {
	Messaging   MessagingConfig   `yaml:"messaging"`
	Persistence PersistenceConfig `yaml:"persistence"`
	Runtime     RuntimeConfig     `yaml:"runtime"`
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

// Default returns production defaults (NATS + Postgres).
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
	}
}

// Dev returns in-memory adapters for local development.
// Set AGENTOS_WORKER_ADDR to enable execution (e.g. localhost:50051).
func Dev() Config {
	addr := os.Getenv("AGENTOS_WORKER_ADDR")
	return Config{
		Runtime:     RuntimeConfig{WorkerAddr: addr},
		Messaging:   MessagingConfig{Provider: "memory"},
		Persistence: PersistenceConfig{Provider: "memory"},
	}
}
