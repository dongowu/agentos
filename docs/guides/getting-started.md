# Getting Started

This guide expands on the homepage quick start and helps you pick the right local path for AgentOS.

## Choose a Local Run Path

### 1. Fastest substrate check

Use this when you want to prove the local execution path works before turning on planning or full control-plane topology.

```bash
# Terminal 1: start the Rust worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: submit a task through the local control path
export AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051
go run ./cmd/osctl submit "echo hello"
```

### 2. Local run with LLM planning

Use this when you want the same execution substrate plus planner / agent-loop behavior.

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051 \
       AGENTOS_LLM_PROVIDER=openai \
       AGENTOS_LLM_API_KEY=sk-xxx \
       AGENTOS_LLM_BASE_URL=https://api.openai.com \
       AGENTOS_LLM_MODEL=gpt-4o
go run ./cmd/osctl submit "create a hello world python script"
```

### 3. Full multiprocess acceptance

Use this when you want the real `controller + apiserver + worker` acceptance loop, including scheduling, auth, audit, replay, and the control-plane bridge.

```bash
./scripts/acceptance.sh
```

## Key Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `AGENTOS_MODE` | `dev` uses in-memory adapters and local scheduler; `prod` uses NATS and Postgres paths | `prod` |
| `AGENTOS_WORKER_ADDR` | Direct Rust worker gRPC address for local / dev execution | `localhost:50051` |
| `AGENTOS_CONTROL_PLANE_ADDR` | Shared controller registry address for remote worker discovery | — |
| `AGENTOS_SCHEDULER_MODE` | `local` or `nats` | prod: `nats`, dev: `local` |
| `AGENTOS_NATS_URL` | NATS address for messaging and scheduling | `nats://localhost:4222` |
| `AGENTOS_NATS_STREAM` | JetStream stream name | `AGENTOS` |
| `AGENTOS_POSTGRES_DSN` | Postgres DSN for persistence adapters | `postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable` |
| `AGENTOS_API_LISTEN_ADDR` | API listen address | `:8080` |
| `AGENTOS_LLM_PROVIDER` | Planner provider name from the adapter registry | `openai` when configured in dev, otherwise stub fallback |
| `AGENTOS_LLM_API_KEY` | LLM provider API key | — |
| `AGENTOS_LLM_BASE_URL` | OpenAI-compatible base URL | `https://api.openai.com` |
| `AGENTOS_LLM_MODEL` | LLM model name | `gpt-4o` |
| `AGENTOS_RUNTIME` | Worker runtime adapter | `native` |
| `AGENTOS_SECURITY_LEVEL` | `supervised`, `semi`, or `autonomous` | `supervised` |
| `AGENTOS_DOCKER_IMAGE` | Docker sandbox image | `ubuntu:22.04` |
| `AGENTOS_MAX_CONCURRENT_TASKS` | Worker concurrency limit | `4` |
| `AGENTOS_AGENT_SECRETS` | Agent secret mapping used for opaque token injection | — |
| `AGENTOS_AUTH_TOKENS` | Bearer auth mapping for `/v1/tasks*` and gateway routes | — |

## Validate the Stack

```bash
# Go tests
go test ./...

# Rust tests
cd runtime && cargo test --workspace

# Real multiprocess acceptance
./scripts/acceptance.sh
```

## Read Next

- [Core Capabilities Reference](../reference/core-capabilities.md)
- [Architecture Overview](../architecture/overview.md)
- [Multiprocess Acceptance](../architecture/multiprocess-acceptance.md)
- [Documentation Index](../README.md)
