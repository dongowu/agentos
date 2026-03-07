# AgentOS

**AgentOS = Kubernetes for AI Agents**

An open-source Agent execution platform with a Go control plane and a Rust runtime plane. LLMs no longer call tools directly — they run on AgentOS.

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
| **Agent Brain** | LLM Planner (OpenAI-compatible), Agent YAML DSL | Implemented |
| **Task Engine** | State machine, lifecycle transitions | Implemented |
| **Skill System** | Tool registry, 7 built-in tools, SchemaAware | Implemented |
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
       AGENTOS_LLM_API_KEY=sk-xxx \
       AGENTOS_LLM_BASE_URL=https://api.openai.com \
       AGENTOS_LLM_MODEL=gpt-4o
go run ./cmd/osctl submit "create a hello world python script"
```

The HTTP gateway exposes `/health`, `/agent/run`, `/agent/status`, `/agent/list`, `/tool/run`, `GET /v1/tasks/{task_id}/stream` for task-level SSE telemetry, `GET /v1/tasks/{task_id}/actions/{action_id}/stream` for live action stdout/stderr streaming, `GET /v1/tasks/{task_id}/audit` for task audit history, and `GET /v1/tasks/{task_id}/actions/{action_id}/audit` for a single persisted action audit record.

Task telemetry now includes live `task.action.output` chunk events. Native and Docker runtimes both support incremental stream delivery. Once an action finishes, the audit store persists the final command, exit code, worker id, stdout, and stderr so the action stream can replay a completed run snapshot and the audit endpoints can serve durable execution records.

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
| `Planner` | prompt, openai (LLM) | prompt fallback; OpenAI + retry/fallback when configured |
| `Memory.Provider` | inmemory, redis | inmemory |
| `RuntimeAdapter` (Rust) | native, docker | native |
| `Scheduler` | local, nats | nats (prod) / local (dev) |

```go
app, err := bootstrap.FromEnv(ctx)
// AGENTOS_MODE=dev  -> memory adapters + local scheduler + prompt planner
// AGENTOS_MODE=prod -> nats + postgres + nats scheduler + OpenAI planner when API key is set
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
| `AGENTOS_API_LISTEN_ADDR` | API server listen address | :8080 |
| `AGENTOS_LLM_API_KEY` | LLM API key (enables OpenAI planner pipeline) | — |
| `AGENTOS_LLM_BASE_URL` | LLM API base URL | https://api.openai.com |
| `AGENTOS_LLM_MODEL` | LLM model name | gpt-4o |
| `AGENTOS_RUNTIME` | Worker runtime: native or docker | native |
| `AGENTOS_SECURITY_LEVEL` | supervised, semi, autonomous | supervised |
| `AGENTOS_DOCKER_IMAGE` | Docker image for container sandbox | ubuntu:22.04 |
| `AGENTOS_MAX_CONCURRENT_TASKS` | Worker concurrency limit | 4 |
| `AGENTOS_AGENT_SECRETS` | Agent secret map (`agent=secret,agent2=secret2`) for opaque token injection | — |
| `AGENTOS_AUTH_TOKENS` | Bearer auth map (`token=subject|tenant|role1;role2`) for `/v1/tasks*` endpoints | — |

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
| Stage 5: Platform | Web UI (Claw Studio), SDK, Agent Marketplace | Planned |

## Documentation

- [Architecture Overview](docs/architecture/overview.md)
- [AgentOS v1 Architecture](docs/architecture/agentos-v1-architecture.md)
- [ClawOS v1 Architecture](docs/architecture/clawos-v1-architecture.md)
- [Monorepo Structure](docs/architecture/monorepo-structure.md)
- [Pluggable Adapters](docs/architecture/adapters.md)
- [Skill System](docs/architecture/skill-system.md)
- [Policy Engine](docs/architecture/policy-engine.md)
- [MVP Scope](docs/architecture/mvp-scope.md)
- [Bootstrap Plan](docs/plans/2026-03-06-agentos-bootstrap-plan.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

AgentOS is open source. See individual files for license information.
