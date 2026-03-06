# AgentOS Deploy

Local development infrastructure for the default adapters (NATS + PostgreSQL).

## Services

- **NATS** (4222, 8222) - JetStream for EventBus (default adapter)
- **PostgreSQL** (5432) - State store for TaskRepository (default adapter)

## Usage

```bash
# Start NATS and Postgres (required when not using AGENTOS_MODE=dev)
docker compose -f deploy/docker-compose.yml up -d

# Run with default adapters
go run ./cmd/osctl submit "echo hello"
```

## Dev Mode

Without Docker, use in-memory adapters:

```bash
AGENTOS_MODE=dev go run ./cmd/osctl submit "echo hello"
```
