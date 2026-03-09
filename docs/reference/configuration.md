# Configuration Reference

This page documents the current configuration surface for AgentOS.

It focuses on the configuration path that the public binaries actually use today:

- `bootstrap.FromEnv(ctx)`
- `config.Default()` or `config.Dev()`
- `config.ApplyEnvOverrides(...)`

## Configuration Model

The current bootstrap path is primarily **environment-driven**.

At startup:

1. if `AGENTOS_MODE=dev`, the app starts from `config.Dev()`
2. otherwise it starts from `config.Default()`
3. environment overrides are applied on top via `config.ApplyEnvOverrides(...)`

This means the main public control-plane entrypoints currently do **not** load a general platform YAML file.

## Mode Baselines

### Production baseline

`config.Default()` uses these defaults:

| Area | Default |
|------|---------|
| worker address | `localhost:50051` |
| messaging provider | `nats` |
| NATS URL | `nats://localhost:4222` |
| NATS stream | `AGENTOS` |
| persistence provider | `postgres` |
| Postgres DSN | `postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable` |
| LLM provider | `openai` |
| LLM model | `gpt-4o` |
| LLM base URL | `https://api.openai.com` |
| memory provider | `inmemory` |
| memory TTL | `24h` |
| default autonomy | `supervised` |
| scheduler mode | `nats` |
| scheduler heartbeat timeout | `30s` |
| scheduler health check interval | `10s` |
| agent dir | `agents` |

### Development baseline

`config.Dev()` uses these defaults:

| Area | Default |
|------|---------|
| worker address | taken from `AGENTOS_WORKER_ADDR` if set |
| messaging provider | `memory` |
| persistence provider | `memory` |
| LLM provider | `stub` by default, auto-switches to `openai` when `AGENTOS_LLM_API_KEY` is present |
| LLM model | `gpt-4o` |
| LLM base URL | `https://api.openai.com` |
| memory provider | `inmemory` |
| default autonomy | `autonomous` |
| scheduler mode | `local` |
| scheduler heartbeat timeout | `30s` |
| scheduler health check interval | `10s` |

## Planner Selection Notes

Current planner behavior depends on both provider support and API key presence:

- if no usable LLM provider is available, bootstrap falls back to `PromptPlanner`
- if `openai` is configured **without** an API key, bootstrap still falls back to `PromptPlanner`
- if a supported provider and API key are available, bootstrap uses an LLM planner wrapped in retry + fallback behavior
- unsupported provider names also fall back to `PromptPlanner`

## Environment Variables

### Control-Plane Bootstrap

These variables are read by `pkg/config` and `internal/bootstrap`.

| Variable | Purpose | Default / Behavior |
|----------|---------|--------------------|
| `AGENTOS_MODE` | choose bootstrap baseline | unset = production baseline, `dev` = development baseline |
| `AGENTOS_WORKER_ADDR` | direct Rust worker gRPC address | prod default `localhost:50051`; in dev it seeds local worker path |
| `AGENTOS_CONTROL_PLANE_ADDR` | shared controller registry address | unset by default |
| `AGENTOS_SCHEDULER_MODE` | scheduler mode override | baseline decides `nats` or `local` |
| `AGENTOS_NATS_URL` | NATS connection URL | `nats://localhost:4222` |
| `AGENTOS_NATS_STREAM` | JetStream stream name | `AGENTOS` |
| `AGENTOS_POSTGRES_DSN` | Postgres DSN | `postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable` |
| `AGENTOS_LLM_PROVIDER` | planner provider name | prod baseline `openai`; dev baseline `stub` unless API key is present |
| `AGENTOS_LLM_API_KEY` | LLM API key | unset by default |
| `AGENTOS_LLM_BASE_URL` | OpenAI-compatible base URL | `https://api.openai.com` |
| `AGENTOS_LLM_MODEL` | LLM model name | `gpt-4o` |
| `AGENTOS_AGENT_SECRETS` | agent secret mapping | parsed as `agent=secret,agent2=secret2` |
| `AGENTOS_AUTH_TOKENS` | bearer token mapping | parsed as `token=subject\|tenant\|role1;role2,...` |

### HTTP API Server

| Variable | Purpose | Default |
|----------|---------|---------|
| `AGENTOS_API_LISTEN_ADDR` | `apiserver` listen address | `:8080` |

### CLI Variables

| Variable | Consumer | Purpose | Default |
|----------|----------|---------|---------|
| `AGENTOS_SERVER_URL` | `osctl` | remote AgentOS API base URL | empty = local embedded mode |
| `AGENTOS_AUTH_TOKEN` | `osctl`, `claw-cli` | bearer token for authenticated servers | unset |

### Controller

| Variable | Purpose | Default |
|----------|---------|---------|
| `GRPC_LISTEN_ADDR` | controller gRPC listen address for worker registry | `:50052` |

### Rust Worker Runtime

These variables are read by `runtime/crates/worker/src/config.rs`.

| Variable | Purpose | Default |
|----------|---------|---------|
| `AGENTOS_LISTEN_ADDR` | worker gRPC listen address | `127.0.0.1:50051` |
| `AGENTOS_WORKER_ID` | unique worker id | hostname-random suffix |
| `AGENTOS_CONTROL_PLANE_ADDR` | controller address used for registration | unset |
| `AGENTOS_HEARTBEAT_INTERVAL_SECS` | heartbeat interval in seconds | `10` |
| `AGENTOS_MAX_CONCURRENT_TASKS` | worker concurrency limit | `4` |
| `AGENTOS_RUNTIME` | runtime kind | `native` |
| `AGENTOS_DOCKER_IMAGE` | docker runtime image | `ubuntu:22.04` |
| `AGENTOS_DOCKER_MEMORY_MB` | docker memory limit | `512` |
| `AGENTOS_DOCKER_CPU_LIMIT` | docker CPU limit | `1.0` |
| `AGENTOS_DOCKER_NETWORK` | docker network mode | `none` |
| `AGENTOS_DOCKER_MOUNT_WORKSPACE` | mount host workspace into docker runtime | `false` |
| `AGENTOS_DOCKER_READ_ONLY` | use read-only docker rootfs | `false` |
| `AGENTOS_SECURITY_LEVEL` | autonomy mode | `supervised` |
| `AGENTOS_MAX_ACTIONS_PER_HOUR` | action rate limit | `120` |
| `AGENTOS_MAX_OUTPUT_BYTES` | output truncation limit | `1048576` |
| `AGENTOS_FORBIDDEN_PATHS` | comma-separated forbidden paths | security-policy default |

### Test / Integration Variables

These are useful for tests and local verification, but they are not part of the normal production bootstrap surface:

| Variable | Purpose |
|----------|---------|
| `AGENTOS_TEST_POSTGRES_DSN` | postgres integration tests |
| `AGENTOS_REDIS_ADDR` | redis integration tests |

## Encoded Value Formats

### `AGENTOS_AGENT_SECRETS`

Format:

```text
agent-a=secret-a,agent-b=secret-b
```

Meaning:

- each entry is `agent_name=secret`
- invalid or empty entries are ignored
- these secrets are loaded into the credential vault mapping used by the control plane

### `AGENTOS_AUTH_TOKENS`

Format:

```text
token-a=user-a|tenant-a|admin;writer,token-b=user-b|tenant-b
```

Meaning:

- each entry is `token=subject|tenant|role1;role2`
- `subject` is required
- `tenant` and `roles` are optional
- roles are separated with `;`

## Practical Profiles

### Fast local development

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051
```

What this gives you:

- memory messaging
- memory persistence
- local scheduler
- no external NATS or Postgres dependency

### Local development with LLM planning

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051 \
       AGENTOS_LLM_PROVIDER=openai \
       AGENTOS_LLM_API_KEY=sk-xxx \
       AGENTOS_LLM_BASE_URL=https://api.openai.com \
       AGENTOS_LLM_MODEL=gpt-4o
```

### Multiprocess local control plane

```bash
export AGENTOS_API_LISTEN_ADDR=127.0.0.1:18080
export GRPC_LISTEN_ADDR=127.0.0.1:15052
export AGENTOS_CONTROL_PLANE_ADDR=127.0.0.1:15052
```

### Docker-backed worker

```bash
export AGENTOS_RUNTIME=docker \
       AGENTOS_DOCKER_IMAGE=ubuntu:22.04 \
       AGENTOS_DOCKER_NETWORK=none \
       AGENTOS_DOCKER_MEMORY_MB=512 \
       AGENTOS_DOCKER_CPU_LIMIT=1.0
```

## What Is Not Yet A Stable Env Surface

The `Config` struct contains additional fields, but the main `FromEnv` bootstrap path does not currently expose all of them as first-class environment variables.

Examples include:

- memory provider switching beyond the default mode baselines
- memory TTL / Redis address / Redis prefix on the public env surface
- policy rule lists from env
- agent directory selection from env
- scheduler timing overrides via env

Those knobs currently require constructing `bootstrap.New(ctx, cfg)` directly in code instead of relying only on `bootstrap.FromEnv(ctx)`.

## Read Next

- [API Surfaces Reference](api-surfaces.md)
- [Core Capabilities Reference](core-capabilities.md)
- [Runtime And Sandbox Reference](runtime-and-sandbox.md)
- [Getting Started Guide](../guides/getting-started.md)
