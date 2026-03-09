# AgentOS

Open-source agent execution platform with a Go control plane and a Rust runtime plane.

AgentOS helps teams run agents as governed workloads instead of ad-hoc scripts and isolated demos.

## What AgentOS Is

The public repository focuses on the **community core**:

- self-hosted control plane and worker runtime
- task orchestration and execution lifecycle
- local and NATS-backed scheduling paths
- audit, replay, and SSE telemetry APIs
- agent loop / tool-calling support
- extension points for tools, skills, and adapters

## Who It's For

AgentOS is best suited for:

- platform and infrastructure teams building internal agent platforms
- engineering productivity teams running developer-facing automation
- operations and workflow automation teams that need audit and execution control
- builders who want a self-hosted execution substrate rather than a chat-first app shell

AgentOS is **not** currently positioned as a polished end-user chat product or a complete enterprise console.

## Why AgentOS

Most agent projects are good at prompts and demos, but weak at execution infrastructure.

| Need | What AgentOS Provides |
|------|-----------------------|
| Safe execution | Rust worker runtime, sandboxing paths, policy hooks, secret isolation |
| Operational control | task lifecycle, scheduling, worker registry, replayable execution records |
| Observability | audit trail, SSE telemetry, action output streaming |
| Extensibility | tools, skills, adapters, and control-plane bridge surfaces |
| Self-hosting | community core that teams can run in their own environment |

## Quick Start

Choose the path that matches your goal.

### 1. Fastest local run

```bash
# Terminal 1: start the Rust worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: submit a task through the local control path
export AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051
go run ./cmd/osctl submit "echo hello"
```

This is the fastest way to verify the execution substrate locally.

### 2. Local run with LLM planning

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051 \
       AGENTOS_LLM_PROVIDER=openai \
       AGENTOS_LLM_API_KEY=sk-xxx \
       AGENTOS_LLM_BASE_URL=https://api.openai.com \
       AGENTOS_LLM_MODEL=gpt-4o
go run ./cmd/osctl submit "create a hello world python script"
```

This enables the LLM-backed planner / agent loop path on top of the same execution substrate.

### 3. Full multiprocess acceptance

```bash
./scripts/acceptance.sh
```

This validates the real `controller + apiserver + worker` flow, including auth, scheduling, audit, replay, and control-plane bridge behavior.

## Architecture

```text
Clients (CLI / API / SDK)
  -> Access Layer (Go)
  -> Orchestration Core (Go)
  -> Scheduler / Worker Registry (Go)
  -> Execution Workers (Rust)
  -> Sandbox / Tool Surfaces
```

At a high level:

- `apiserver` exposes HTTP, audit, replay, and SSE telemetry APIs
- `controller` handles shared worker registration and control-plane coordination
- the orchestration core manages task state, planning, policy, and dispatch
- workers execute actions through native or container-backed runtime paths
- tool-like actions can also run through the Go control-plane bridge when appropriate

## Core Systems

| System | Responsibility | Status |
|--------|---------------|--------|
| **Access** | HTTP API, CLI, Gateway | Implemented |
| **Agent Brain** | Registry-backed LLM Planner (OpenAI-compatible), Agent YAML DSL | Implemented |
| **Task Engine** | State machine, lifecycle transitions | Implemented |
| **Skill System** | Tool registry, built-in tools, SchemaAware, action bridge for file/http-style actions | Implemented |
| **Policy Engine** | Allow/deny rules, autonomy levels, credential isolation | Implemented |
| **Runtime** | Rust Worker, NativeRuntime, DockerRuntime, SecurityPolicy | Implemented |
| **Scheduler** | Worker registry, health monitor, NATS queue, worker pool | Implemented |
| **Audit** | Platform audit store with persistent task/action records | Implemented |
| **Memory** | In-memory + Redis providers, TTL support | Implemented |

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

- [Documentation Guide](docs/README.md)
- [Changelog](CHANGELOG.md)
- [Architecture Overview](docs/architecture/overview.md)
- [Multiprocess Acceptance](docs/architecture/multiprocess-acceptance.md)
- [Licensing Decision](docs/strategy/licensing-decision.md)
- [Platform vs Capability Boundary](docs/architecture/platform-vs-capability-boundary.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and contribution guidelines, [SUPPORT.md](SUPPORT.md) for community support boundaries, [SECURITY.md](SECURITY.md) for vulnerability reporting, and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for community expectations.

## License

AgentOS core is licensed under [Apache-2.0](LICENSE). Enterprise add-ons and hosted cloud offerings may be distributed under separate commercial terms.
