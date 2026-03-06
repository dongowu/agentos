# AgentOS

AgentOS is a hybrid autonomous execution platform with a Go control plane and a Rust runtime plane.

The design goal is simple: keep planning, orchestration, and persistence in Go; keep isolation, execution, and telemetry in Rust; connect both sides through stable contracts so each piece stays replaceable.

## Architecture

**AgentOS = Kubernetes for AI Agents**

Agent Infrastructure：LLM 不再直接调用工具，而是运行在 AgentOS 上。

6 大核心系统：Access → Agent Brain → Task Engine → Skill System → Policy Engine → Runtime.

详见 [AgentOS v1 Architecture](docs/architecture/agentos-v1-architecture.md)。

## Documentation

- [ClawOS v1 架构](docs/architecture/clawos-v1-architecture.md) - 一步到位，可直接开干
- [AgentOS v1 Architecture](docs/architecture/agentos-v1-architecture.md) - 完整架构定型
- [Monorepo 最终版结构](docs/architecture/monorepo-structure.md) - 10 万行可扩展目录
- [Architecture Overview](docs/architecture/overview.md)
- [Pluggable Adapters](docs/architecture/adapters.md)
- [Skill System](docs/architecture/skill-system.md)
- [Policy Engine](docs/architecture/policy-engine.md)
- [MVP Scope](docs/architecture/mvp-scope.md)
- [Bootstrap Plan](docs/plans/2026-03-06-agentos-bootstrap-plan.md)

## Repository Layout

```text
agentos/
├─ api/
│  └─ proto/
├─ cmd/
│  ├─ apiserver/
│  ├─ controller/
│  └─ osctl/
├─ docs/
│  ├─ architecture/
│  └─ plans/
├─ internal/
│  ├─ access/
│  ├─ adapters/
│  │  ├─ messaging/
│  │  │  ├─ memory/
│  │  │  └─ nats/
│  │  └─ persistence/
│  │     ├─ memory/
│  │     └─ postgres/
│  ├─ bootstrap/
│  ├─ messaging/
│  ├─ orchestration/
│  ├─ persistence/
│  └─ runtimeclient/
├─ pkg/
│  ├─ events/
│  └─ taskdsl/
├─ runtime/
│  └─ crates/
├─ deploy/
└─ examples/
```

## Plug-in Boundaries

The system is intentionally interface-first so the major subsystems remain pluggable.

### Access Layer Interfaces

- `TaskSubmissionAPI`
  - Accepts task creation requests from HTTP or CLI.
  - Returns task IDs and streaming handles.
- `AuthProvider`
  - Validates tokens and resolves tenant identity.
  - Can later support JWT, GitHub OAuth, or Google OAuth.

### Orchestration Layer Interfaces

- `Planner`
  - Converts a prompt into a structured `Plan`.
  - Different implementations can target Gemini, GPT, Claude, or a local model.
- `TaskEngine`
  - Owns task lifecycle transitions and retries.
  - Decides when a task moves from `pending` to `planning`, `queued`, `running`, and final states.
- `SkillResolver`
  - Maps an `Action` to the right execution profile.
  - Lets future skills stay declarative instead of hardcoded in the controller.
- `MemoryProvider`
  - Optional lookup layer for long-term memory and retrieval.
  - Kept as an interface so MVP can ship without vector DB coupling.

### Messaging & Persistence Interfaces

- `EventBus`
  - Publishes and subscribes to domain events.
  - Adapters: `memory`, `nats` (default).
- `TaskRepository`
  - Persists tasks, plans, and execution history.
  - Adapters: `memory`, `postgres` (default).
- `AuditLogStore`
  - Stores command traces, side effects, and execution metadata.
  - Forms the base for future compliance and SaaS observability.

### Execution Layer Interfaces

- `RuntimeBroker`
  - Requests and releases runtime leases for tasks.
  - Shields the Go control plane from Docker, gVisor, or Firecracker details.
- `ExecutorClient`
  - Sends actions to the Rust worker and receives structured results.
  - Will be backed by protobuf and gRPC contracts.
- `IsolationProvider`
  - Rust-side abstraction over the isolation backend.
  - Implementations can be `DockerProvider`, `GVisorProvider`, or `FirecrackerProvider`.
- `TelemetryStreamer`
  - Emits stdout, stderr, resource usage, and screen artifacts.
  - Lets UI and CLI reuse one streaming model.

## MVP Scope

The first milestone proves a single happy-path flow:

1. Submit a task through CLI or HTTP.
2. Convert prompt to a `Plan`.
3. Dispatch a single `Action`.
4. Execute the action in a Rust-managed sandbox.
5. Stream logs back to the control plane.
6. Persist final task state.

The MVP explicitly does not include:

- Browser automation
- Firecracker isolation
- Vector memory
- Web3 transaction signing
- Full dashboard UI
- Multi-model routing complexity

## What Each Part Will Build

### `cmd/apiserver`
- Starts the HTTP and WebSocket API.
- Exposes task submission and task stream endpoints.

### `cmd/controller`
- Runs the orchestration loop.
- Loads planner, repositories, event bus, and runtime client adapters.

### `cmd/osctl`
- Provides a local developer CLI for submitting tasks and watching progress.

### `internal/access`
- HTTP handlers, CLI command wiring, auth middleware, and request DTOs.

### `internal/orchestration`
- Task state machine, planner adapters, skill resolution, and task execution decisions.

### `internal/bootstrap`
- Config-based dependency wiring. Selects EventBus and TaskRepository adapters from `pkg/config`.

### `internal/adapters`
- Pluggable implementations: `messaging/memory`, `messaging/nats`, `persistence/memory`, `persistence/postgres`.

### `internal/messaging`
- EventBus interface. Implementations in `internal/adapters/messaging/`.

### `internal/persistence`
- TaskRepository interface. Implementations in `internal/adapters/persistence/`.

### `internal/runtimeclient`
- Go-side gRPC client that talks to Rust workers through stable contracts.

### `pkg/config`
- Adapter configuration (MessagingConfig, PersistenceConfig). `Default()` and `Dev()` presets.

### `pkg/taskdsl`
- Domain-safe definitions for `Task`, `Plan`, `Action`, and execution metadata.

### `pkg/events`
- Domain events that describe task lifecycle transitions and action results.

### `api/proto`
- Versioned contracts shared between Go and Rust.

### `runtime/crates/worker`
- Rust worker process that receives execution requests and controls sandbox lifecycles.

### `runtime/crates/sandbox`
- Isolation abstractions and backend providers.

### `runtime/crates/telemetry`
- Reusable telemetry models and stream helpers.

## Interface-First Example

```go
type Planner interface {
	Plan(ctx context.Context, input PlanInput) (Plan, error)
}

type RuntimeBroker interface {
	Acquire(ctx context.Context, spec RuntimeSpec) (RuntimeLease, error)
	Release(ctx context.Context, leaseID string) error
}

type EventBus interface {
	Publish(ctx context.Context, event DomainEvent) error
}
```

```rust
#[async_trait::async_trait]
pub trait IsolationProvider {
    async fn start(&self, spec: SandboxSpec) -> Result<SandboxHandle, SandboxError>;
    async fn stop(&self, sandbox_id: &str) -> Result<(), SandboxError>;
}
```

These interfaces are the contract seams that keep AgentOS modular.

## Plugin Architecture

The system uses a **pluggable architecture + built-in default adapters**. Interfaces live in `internal/messaging` and `internal/persistence`; implementations live in `internal/adapters`.

### Bootstrap

`internal/bootstrap` wires adapters from `pkg/config`:

```go
app, err := bootstrap.FromEnv(ctx)
// AGENTOS_MODE=dev  -> memory adapters
// otherwise         -> NATS + Postgres (default)
```

Or with explicit config:

```go
cfg := config.Default()  // nats + postgres
cfg := config.Dev()     // memory for both
app, err := bootstrap.New(ctx, cfg)
```

### Built-in Adapters

| Interface | Provider | Package |
|-----------|----------|---------|
| `EventBus` | `memory` | `internal/adapters/messaging/memory` |
| `EventBus` | `nats` | `internal/adapters/messaging/nats` |
| `TaskRepository` | `memory` | `internal/adapters/persistence/memory` |
| `TaskRepository` | `postgres` | `internal/adapters/persistence/postgres` |

**Defaults:** `messaging=nats`, `persistence=postgres`. Set `AGENTOS_MODE=dev` to use in-memory adapters for local development.

### Adding a Custom Adapter

1. Implement the interface in a new package under `internal/adapters/`.
2. Add a case in `bootstrap.newEventBus` or `bootstrap.newTaskRepository`.
3. Extend `pkg/config` if the adapter needs new config fields.

## Framework Status

The repository now has the initial project framework in place:

- `README`, architecture docs, contribution guide, and execution plan
- Go monorepo layout with `cmd`, `internal`, and `pkg` boundaries
- Core domain types for `Task`, `Plan`, `Action`, and lifecycle events
- Interface-first control-plane contracts for access, orchestration, messaging, persistence, and runtime clients
- Versioned protobuf contracts under `api/proto/agentos/v1`
- Rust workspace with `worker`, `sandbox`, and `telemetry` crates
- Local development infrastructure with NATS and PostgreSQL compose stubs
- CI skeleton for Go and Rust test execution

## Next Build Steps

The next implementation layers should be added on top of this framework in order:

1. ~~Wire `TaskSubmissionAPI` into HTTP and CLI adapters.~~ (done)
2. ~~Make `TaskEngine` use `Planner`, `SkillResolver`, `TaskRepository`, and `EventBus` together.~~ (done)
3. ~~Replace in-memory adapters with NATS and PostgreSQL implementations.~~ (done: pluggable adapters, default nats+postgres)
4. ~~Expose `WorkerService` through a real gRPC server in Rust.~~ (done)
5. ~~Add one end-to-end happy path from task submission to action completion.~~ (done)

## End-to-End Flow

```bash
# Terminal 1: Start the Rust worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: Submit a task (with worker)
$env:AGENTOS_MODE='dev'; $env:AGENTOS_WORKER_ADDR='localhost:50051'
go run ./cmd/osctl submit "echo hello"
# Output: task task-xxx created (state: succeeded)
```
