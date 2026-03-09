# AgentOS

**AgentOS = Kubernetes for AI Agents**

An open-source agent execution platform with a Go control plane and a Rust runtime plane. LLMs no longer call tools directly — they run on AgentOS.

Safe. Controllable. Extensible. Distributed.

## Why AgentOS

Current Agent frameworks (LangChain, AutoGPT, CrewAI) are application-layer glue code. They suffer from four critical gaps:

| Problem | AgentOS Solution |
|---------|-----------------|
| **Insecure** — AI-generated code runs with host privileges | Rust sandbox with Docker/gVisor isolation, env isolation, secret redaction |
| **Unscalable** — Fragmented tool ecosystems, no standard ABI | Pluggable Tool interface, 7 built-in tools, OpenClaw skill compatibility |
| **Unobservable** — Execution is a black box | Event-driven audit trail, telemetry streaming, policy logging |
| **Not production-ready** — Single-machine demos only | NATS-based distributed scheduling, multi-worker pool, auto-scaling |

## Architecture

```
            [ Client (CLI / API / UI / SDK) ]
                          |
  +-----------------------v-----------------------+
  |              Access Layer (Go)                |
  |        HTTP Gateway + CLI + Auth              |
  +-----------------------+-----------------------+
                          |
  +-----------------------v-----------------------+
  |          Orchestration Core (Go)              |
  |                                               |
  |  [LLM Planner]  [Task Engine]  [Scheduler]   |
  |  [Skill Resolver]  [Policy Engine]            |
  +-----------------------+-----------------------+
                          |
  +-----------------------v-----------------------+
  |         Worker Pool + Registry (Go)           |
  |  [Registry]  [Health Monitor]  [Pool]         |
  +-----------------------+-----------------------+
                          |
  +-----------------------v-----------------------+
  |         Execution Workers (Rust)              |
  |                                               |
  |  [RuntimeAdapter]  [SecurityPolicy]           |
  |  [ActionExecutor]  [Registration]             |
  +-----------------------+-----------------------+
                          |
  +-----------------------v-----------------------+
  |           Execution Sandbox                   |
  |     [Native]    [Docker]    [WASM (future)]   |
  +-----------+-----------------------------------+
              |
  +-----------v-----------------------------------+
  |            Tools Ecosystem                    |
  |  shell / file / git / http / browser (future) |
  +-----------------------------------------------+
```

## Core Systems

| System | Responsibility | Status |
|--------|---------------|--------|
| **Access** | HTTP API, CLI, Gateway | Implemented |
| **Agent Brain** | Registry-backed LLM Planner (OpenAI-compatible), Agent YAML DSL | Implemented |
| **Task Engine** | State machine, lifecycle transitions | Implemented |
| **Skill System** | Tool registry, 7 built-in tools, SchemaAware, action bridge for file/http-style actions | Implemented |
| **Policy Engine** | Allow/deny rules, autonomy levels, credential isolation | Implemented |
| **Runtime** | Rust Worker, NativeRuntime, DockerRuntime, SecurityPolicy | Implemented |
| **Scheduler** | Worker registry, health monitor, NATS queue, worker pool | Implemented |
| **Audit** | Platform audit store with persistent task/action records | Implemented |
| **Memory** | In-memory + Redis providers, TTL support | Implemented |

## Quick Start

```bash
# Terminal 1: Start the Rust worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: Submit a task
export AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051
go run ./cmd/osctl submit "echo hello"
# Output: task task-xxx created (state: succeeded)
```

This direct-worker flow is the supported local path for `osctl` and `apiserver` when `AGENTOS_WORKER_ADDR` is set. In this mode, the control plane falls back to the configured worker address if no local scheduler worker is available.

`controller` is still useful for worker registration and health monitoring, but it is a separate multi-process path from the direct local quick start above.

With LLM planning:

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051 \
       AGENTOS_LLM_PROVIDER=openai \
       AGENTOS_LLM_API_KEY=sk-xxx \
       AGENTOS_LLM_BASE_URL=https://api.openai.com \
       AGENTOS_LLM_MODEL=gpt-4o
go run ./cmd/osctl submit "create a hello world python script"
```

The planner backend is registry-driven. `openai` is built in, and other OpenAI-compatible or future providers can be added through the same adapter registry without changing bootstrap wiring.

For remote API usage with `osctl`:

```bash
export AGENTOS_AUTH_TOKEN=acceptance-token
go run ./cmd/osctl --server http://localhost:8080 submit "echo hello"
go run ./cmd/osctl --server http://localhost:8080 --token acceptance-token status task-123
```

When `--server` (or `AGENTOS_SERVER_URL`) is set, `osctl` skips local bootstrap and talks to the remote `/v1/tasks` API directly. `--token` defaults from `AGENTOS_AUTH_TOKEN`. Leaving `--server` empty keeps the existing embedded local mode.

For local agent iteration with the gateway CLI:

```bash
claw --server http://localhost:8080 dev
claw --server http://localhost:8080 dev agents/demo.yaml "echo hello"

# against an authenticated gateway
export AGENTOS_AUTH_TOKEN=acceptance-token
claw --server http://localhost:8080 dev
claw --server http://localhost:8080 run agents/demo.yaml "echo hello"
claw --server http://localhost:8080 --token acceptance-token status task-123
```

The zero-arg form checks `/health` and `/agent/list`; the two-arg form loads the local `agent.yaml` and submits it through `/agent/run`. When the gateway requires Bearer auth, `claw-cli` accepts `--token` or `AGENTOS_AUTH_TOKEN`.

The HTTP gateway exposes `/health`, `/agent/run`, `/agent/status`, `/agent/list`, `/tool/run`, `GET /v1/audit` for a platform-level audit feed API, `GET /v1/tasks/{task_id}/stream` for task-level SSE telemetry, `GET /v1/tasks/{task_id}/actions/{action_id}/stream` for live action stdout/stderr streaming, `GET /v1/tasks/{task_id}/audit` for task audit history, `GET /v1/tasks/{task_id}/actions/{action_id}/audit` for a single persisted action audit record, and `GET /v1/tasks/{task_id}/replay` for a task-centric replay projection that joins task metadata, planned actions, and persisted audit results. When bearer auth is configured, both `/v1/tasks*` and gateway routes require the same `Authorization: Bearer ...` header. For `/agent/run`, the control plane now preserves `agent_name` and lets the loaded agent profile shape the planner prompt instead of forwarding only the raw task text.

Task telemetry now includes live `task.action.output` chunk events. Native and Docker runtimes both support incremental stream delivery. Once an action finishes, the audit store persists the final command, exit code, worker id, stdout, and stderr so the action stream can replay a completed run snapshot, the task stream can backfill persisted action output/completion events for already-finished runs, `/v1/audit` can expose a tenant-scoped global audit feed, and the audit/replay endpoints can serve durable execution records. This is an API-level audit center surface, not yet a full console UI.

The LLM planner now treats malformed plan output as a structured recovery path: it first asks the provider to repair the bad JSON once, returns `ErrMalformedPlan` if repair still fails, and lets the outer fallback planner hand execution to `PromptPlanner` instead of burning more retries on non-transient output errors.

Tool-like actions no longer need to be downgraded into shell commands to run. `file.read`, `file.write`, direct tool kinds, and `http.request` (currently bridged to `http.get` / `http.post`) can execute through the Go control-plane tool bridge, while `command.exec` still goes through the Rust worker. Browser-specialized execution is still a future gap.

For a real three-process acceptance run (`controller + apiserver + worker`) including audit verification, use:

```bash
./scripts/acceptance.sh
```

## Agent DSL

Agents are config, not code:

```yaml
name: defi-trading-agent
description: "Monitors markets and executes trades."
model: gpt-4o

memory:
  type: redis
  ttl: 86400

tools:
  - http.get
  - http.post
  - shell

policy:
  allow: ["http.*"]
  deny: ["shell"]

workflow:
  - plan
  - execute
  - reflect
```

## Built-in Tools

| Tool | Description |
|------|------------|
| `shell` | Execute shell commands (with sandbox) |
| `file.read` | Read file contents |
| `file.write` | Write files (auto-creates dirs) |
| `git.clone` | Clone git repositories |
| `git.status` | Get git status |
| `http.get` | HTTP GET requests |
| `http.post` | HTTP POST requests |

## Repository Layout

```
agentos/
├── api/
│   ├── proto/agentos/v1/          # Protobuf contracts (Go <-> Rust)
│   └── gen/                       # Generated Go gRPC code
├── cmd/
│   ├── apiserver/                 # HTTP + SSE API server
│   ├── controller/                # Orchestration loop
│   ├── claw-cli/                  # ClawOS CLI
│   └── osctl/                     # Developer CLI
├── internal/
│   ├── access/                    # HTTP handlers, CLI wiring, auth
│   ├── adapters/
│   │   ├── llm/openai/            # OpenAI-compatible LLM adapter
│   │   ├── memory/{inmemory,redis}/ # Memory providers
│   │   ├── messaging/{memory,nats}/ # EventBus adapters
│   │   ├── persistence/{memory,postgres}/ # TaskRepository adapters
│   │   └── runtimeclient/         # gRPC executor client
│   ├── agent/                     # Agent YAML DSL, runtime, manager
│   ├── bootstrap/                 # Dependency wiring from config
│   ├── gateway/                   # HTTP API (/agent/run, /tool/run)
│   ├── memory/                    # Memory interface + builder factory
│   ├── orchestration/             # TaskEngine, Planner, StateMachine
│   ├── policy/                    # PolicyEngine, rules, credential vault
│   ├── scheduler/                 # Local + NATS task schedulers
│   ├── tool/builtin/              # 7 built-in tools (shell,file,git,http)
│   └── worker/                    # Registry, health monitor, pool
├── pkg/
│   ├── config/                    # All system configuration
│   ├── events/                    # Domain events
│   └── taskdsl/                   # Task, Plan, Action types
├── runtime/
│   └── crates/
│       ├── worker/                # Rust gRPC worker + executor
│       ├── sandbox/               # RuntimeAdapter, NativeRuntime, DockerRuntime
│       └── telemetry/             # Streaming telemetry models
├── deploy/                        # Docker Compose (NATS + Postgres)
└── examples/
    ├── agents/                    # Example agent YAML configs
    └── basic-task/
```

## Pluggable Adapters

| Interface | Adapters | Default |
|-----------|----------|---------|
| `EventBus` | memory, nats | nats (prod) / memory (dev) |
| `TaskRepository` | memory, postgres | postgres (prod) / memory (dev) |
| `AuditLogStore` | memory, postgres | postgres (prod) / memory (dev) |
| `Planner` | prompt, registry-backed LLM providers (`openai` built in) | prompt planner baseline; LLM planner does bounded retry for transient failures, one repair pass for malformed JSON, then falls back to `PromptPlanner` |
| `Memory.Provider` | inmemory, redis | inmemory |
| `RuntimeAdapter` (Rust) | native, docker | native |
| `Scheduler` | local, nats | nats (prod) / local (dev) |

```go
app, err := bootstrap.FromEnv(ctx)
// AGENTOS_MODE=dev  -> memory adapters + local scheduler + prompt planner
// AGENTOS_MODE=prod -> nats + postgres + nats scheduler + registry-backed LLM planner when configured
```

## Security Model

**Go Control Plane (inspired by HiClaw):**
- PolicyEngine: allow/deny rules with glob matching, deny-takes-precedence
- AutonomyLevel: Supervised / SemiAutonomous / Autonomous
- CredentialVault: workers get opaque tokens, real secrets only in gateway
- Rate limiting per agent (actions/hour)
- Dangerous command blacklist (rm -rf, dd, mkfs, etc.)

**Rust Worker (inspired by ZeroClaw):**
- SecurityPolicy: command whitelist/blacklist, forbidden paths
- Environment isolation: clear env, re-add only safe vars
- Secret redaction: detects API keys, Bearer tokens, AWS keys in output
- Output truncation: configurable max bytes (default 1MB)
- Timeout enforcement per action
- Docker isolation: --read-only, --network none, resource limits

## Distributed Architecture

```
                    ┌─────────────────┐
                    │   Go Control    │
                    │     Plane       │
                    │                 │
                    │  ┌───────────┐  │
                    │  │ Scheduler │  │
                    │  └─────┬─────┘  │
                    │        │        │
                    │  ┌─────▼─────┐  │
                    │  │  Worker   │  │
                    │  │ Registry  │  │
                    │  └─────┬─────┘  │
                    └────────┼────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼─────┐
        │  Worker 1  │ │  Worker 2  │ │  Worker N  │
        │  (Rust)    │ │  (Rust)    │ │  (Rust)    │
        │ native/    │ │ docker/    │ │ docker/    │
        │ docker     │ │ native     │ │ wasm       │
        └────────────┘ └────────────┘ └────────────┘
```

- Workers register with the control plane on startup
- Periodic heartbeat (default 10s), offline detection (30s timeout)
- Production scheduling uses NATS dispatch/result subjects with a dispatcher bridge
- Dev mode can still execute directly against `AGENTOS_WORKER_ADDR`
- Worker pool uses least-loaded selection plus lazy gRPC connection caching

## Environment Variables

| Variable | Description | Default |
|----------|------------|---------|
| `AGENTOS_MODE` | dev (memory + local scheduler) or prod (nats + postgres) | prod |
| `AGENTOS_WORKER_ADDR` | Direct Rust worker gRPC address for dev/direct execution | localhost:50051 |
| `AGENTOS_CONTROL_PLANE_ADDR` | Shared controller registry address for remote worker discovery | — |
| `AGENTOS_SCHEDULER_MODE` | `local` or `nats` | prod: `nats`, dev: `local` |
| `AGENTOS_NATS_URL` | NATS server URL for messaging/scheduler adapters | nats://localhost:4222 |
| `AGENTOS_NATS_STREAM` | JetStream stream name | AGENTOS |
| `AGENTOS_POSTGRES_DSN` | PostgreSQL DSN for persistence adapters | postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable |
| `AGENTOS_API_LISTEN_ADDR` | API server listen address | :8080 |
| `AGENTOS_LLM_PROVIDER` | Planner provider name resolved from the LLM adapter registry | openai when API key is set in dev, otherwise stub |
| `AGENTOS_LLM_API_KEY` | LLM API key (enables the configured LLM planner pipeline) | — |
| `AGENTOS_LLM_BASE_URL` | LLM API base URL for OpenAI-compatible providers | https://api.openai.com |
| `AGENTOS_LLM_MODEL` | LLM model name | gpt-4o |
| `AGENTOS_RUNTIME` | Worker runtime: native or docker | native |
| `AGENTOS_SECURITY_LEVEL` | supervised, semi, autonomous | supervised |
| `AGENTOS_DOCKER_IMAGE` | Docker image for container sandbox | ubuntu:22.04 |
| `AGENTOS_MAX_CONCURRENT_TASKS` | Worker concurrency limit | 4 |
| `AGENTOS_AGENT_SECRETS` | Agent secret map (`agent=secret,agent2=secret2`) for opaque token injection | — |
| `AGENTOS_AUTH_TOKENS` | Bearer auth map (`token=subject|tenant|role1;role2`) for `/v1/tasks*` and gateway routes | — |

## Test Suite

```bash
# Go tests (13 test suites)
go test ./...

# Rust tests (5 test suites, 69+ tests)
cd runtime && cargo test --workspace
```

## Roadmap

| Stage | Focus | Status |
|-------|-------|--------|
| Stage 1: MVP | Core pipeline (submit -> plan -> execute -> result) | Done |
| Stage 2: Agent System | LLM planner pipeline, Agent YAML DSL, Tools, Memory | Done |
| Stage 3: Sandbox & Policy | Docker isolation, SecurityPolicy, PolicyEngine | Done |
| Stage 4: Distributed | Worker registry, NATS scheduling, worker pool | Done |
| Stage 5: Platform UX | Console, SDK, extension surfaces | Planned |

## Open Core Boundary

AgentOS publishes the repository core under **Apache-2.0** and keeps enterprise/cloud packaging outside the repository-core boundary.

- **Community**: open-source self-hosted control plane, runtime, scheduler, audit and telemetry APIs
- **Enterprise**: org governance, SSO / SCIM / RBAC, audit center, longer retention, support
- **Cloud**: managed control plane, hosted console, upgrades, billing, SLA

For the public repository, the focus is the **Community** core: the self-hosted execution substrate, orchestration contracts, scheduling, audit, and telemetry surfaces. Licensing details live in `docs/strategy/licensing-decision.md`.

## Documentation

- [Architecture Overview](docs/architecture/overview.md)
- [AgentOS v1 Architecture](docs/architecture/agentos-v1-architecture.md)
- [ClawOS v1 Architecture](docs/architecture/clawos-v1-architecture.md)
- [Monorepo Structure](docs/architecture/monorepo-structure.md)
- [Pluggable Adapters](docs/architecture/adapters.md)
- [Skill System](docs/architecture/skill-system.md)
- [Policy Engine](docs/architecture/policy-engine.md)
- [MVP Scope](docs/architecture/mvp-scope.md)
- [Licensing Decision](docs/strategy/licensing-decision.md)
- [Platform vs Capability Boundary](docs/architecture/platform-vs-capability-boundary.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and contribution guidelines, [SUPPORT.md](SUPPORT.md) for community support boundaries, [SECURITY.md](SECURITY.md) for vulnerability reporting, and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for community expectations.

## License

AgentOS core is licensed under [Apache-2.0](LICENSE). Enterprise add-ons and hosted cloud offerings may be distributed under separate commercial terms.
