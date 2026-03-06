# Basic Task Example

Minimal example of submitting a task and receiving a result.

## Dev Mode (no NATS/Postgres, no Worker)

```bash
AGENTOS_MODE=dev go run ./cmd/osctl submit "echo hello"
# Task reaches state: queued (no execution)
```

## Dev Mode with Worker (full E2E)

```bash
# Terminal 1: Start the Rust worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: Submit with worker
$env:AGENTOS_MODE='dev'; $env:AGENTOS_WORKER_ADDR='localhost:50051'
go run ./cmd/osctl submit "echo hello"
# Task reaches state: succeeded
```

## Default Mode (NATS + Postgres + Worker)

```bash
# Start infrastructure
docker compose -f deploy/docker-compose.yml up -d

# Start worker
cd runtime && cargo run -p agentos-worker &

# Submit
go run ./cmd/osctl submit "echo hello"

# Check status (use task ID from submit output)
go run ./cmd/osctl status <task-id>
```
