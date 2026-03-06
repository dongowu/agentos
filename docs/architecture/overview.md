# AgentOS Architecture Overview

AgentOS separates control logic from execution logic and uses a pluggable adapter architecture for messaging and persistence.

## Layers

### 1. Access Layer

Go services that expose HTTP, WebSocket, and CLI entry points.

### 2. Orchestration Layer

Go services that generate plans, resolve skills, and drive the task state machine.

### 3. Messaging and Persistence Layer

Event delivery and state storage are pluggable:

- **EventBus**: `memory` (dev) or `nats` (default, JetStream)
- **TaskRepository**: `memory` (dev) or `postgres` (default)

Adapters live in `internal/adapters/`. `internal/bootstrap` wires them from `pkg/config`.

### 4. Execution Layer

Rust workers manage sandbox startup, command execution, telemetry capture, and runtime isolation.

### 5. Infrastructure Layer

Registry, compute nodes, deployment manifests, and networking primitives that host the platform.

## Design Rule

The control plane should depend on contracts, not concrete runtime implementations.

That means:

- Go code depends on `Planner`, `TaskRepository`, `EventBus`, and `ExecutorClient` interfaces.
- Rust code depends on `IsolationProvider` and telemetry traits.
- Cross-language communication depends only on versioned protobuf contracts.
- Messaging and persistence implementations are swappable via config; defaults are NATS + PostgreSQL.
