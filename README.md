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

### 1. One-command dev bootstrap

```bash
make dev-setup
source .env.agentos.dev
make dev-up
```

Use this when you want the fastest path to a repeatable local development environment with built binaries, a sourceable dev env file, and the full local stack startup flow.

You can also run the underlying scripts directly:

```bash
bash scripts/setup_dev_env.sh --verify-stack
bash scripts/start_full_stack.sh --smoke-test --exit-after-smoke
```

### 2. Fastest local run

```bash
# Terminal 1: start the Rust worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: submit a task through the local control path
export AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051
go run ./cmd/osctl submit "echo hello"
```

This is the fastest way to verify the execution substrate locally.

### 3. Local run with LLM planning

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

### 4. Full multiprocess acceptance

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

## Deeper Reading

If you want details beyond the homepage, start here:

- [Getting Started Guide](docs/guides/getting-started.md)
- [Core Capabilities Reference](docs/reference/core-capabilities.md)
- [API Surfaces Reference](docs/reference/api-surfaces.md)
- [Configuration Reference](docs/reference/configuration.md)
- [Runtime And Sandbox Reference](docs/reference/runtime-and-sandbox.md)
- [Architecture Overview](docs/architecture/overview.md)
- [Multiprocess Acceptance](docs/architecture/multiprocess-acceptance.md)
- [Documentation Index](docs/README.md)
- [Changelog](CHANGELOG.md)

## Open Core Boundary

AgentOS publishes the platform core under `Apache-2.0` and keeps commercial packaging outside the repository boundary.

- **Community** — self-hosted control plane, worker runtime, scheduling, audit APIs, replay, telemetry, and the agent-loop substrate
- **Enterprise (future)** — org governance, SSO / SCIM / RBAC, long-retention audit center, and support workflows
- **Cloud (future)** — hosted control plane, operator console, upgrades, billing, and SLA surfaces

See [Licensing Decision](docs/strategy/licensing-decision.md) and [Platform vs Capability Boundary](docs/architecture/platform-vs-capability-boundary.md) for the current boundary.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and contribution guidelines. See [SUPPORT.md](SUPPORT.md), [SECURITY.md](SECURITY.md), and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for community process and security reporting.

## License

The open-source core is licensed under [Apache-2.0](LICENSE). Enterprise extensions and hosted services can use separate commercial terms.
