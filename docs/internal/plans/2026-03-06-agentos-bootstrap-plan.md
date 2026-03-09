# AgentOS Bootstrap Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the first open-source-ready AgentOS monorepo skeleton that proves the MVP control-plane to runtime execution flow.

**Architecture:** The repository will use a monorepo layout with a Go control plane and a Rust execution plane connected through versioned protobuf contracts. Messaging and persistence use a pluggable adapter architecture: default NATS + PostgreSQL, dev mode uses in-memory adapters.

**Tech Stack:** Go, Rust, Protobuf/gRPC, NATS JetStream, PostgreSQL (pgx), Docker Compose, Cobra, Tonic, Tokio

**Status:** Tasks 1–4 and pluggable adapters completed. See `docs/architecture/adapters.md` for adapter details.

---

### Task 1: Bootstrap the monorepo and repository contract

**Files:**
- Create: `README.md`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `go.mod`
- Create: `runtime/Cargo.toml`
- Create: `docs/architecture/overview.md`
- Create: `docs/architecture/mvp-scope.md`

**Step 1: Write the failing documentation check**

Create `README.md` with a temporary line `TODO` and create a simple validation target in `Makefile`:

```make
.PHONY: verify-readme

verify-readme:
	pwsh -NoProfile -Command "if ((Get-Content README.md -Raw) -match 'TODO') { exit 1 }"
```

**Step 2: Run test to verify it fails**

Run: `make verify-readme`  
Expected: FAIL because `README.md` still contains `TODO`

**Step 3: Write minimal implementation**

Replace the placeholder with a complete bootstrap README that contains:

```md
# AgentOS

Go control plane + Rust runtime for autonomous AI task execution.

## Components
- `cmd/`
- `internal/`
- `api/`
- `runtime/`
- `deploy/`
```

Also add:
- `go.mod` with module path `github.com/<org>/agentos`
- `runtime/Cargo.toml` as a Rust workspace
- `.gitignore` entries for Go, Rust, generated protobuf, `.env`, and local databases
- `docs/architecture/overview.md` describing the five-layer architecture
- `docs/architecture/mvp-scope.md` describing what MVP does and does not include

**Step 4: Run test to verify it passes**

Run: `make verify-readme`  
Expected: PASS

**Step 5: Commit**

```bash
git add README.md Makefile .gitignore go.mod runtime/Cargo.toml docs/architecture/overview.md docs/architecture/mvp-scope.md
git commit -m "chore: bootstrap agentos monorepo structure"
```

### Task 2: Define the Go domain model and task state machine

**Files:**
- Create: `pkg/taskdsl/task.go`
- Create: `pkg/taskdsl/plan.go`
- Create: `pkg/taskdsl/action.go`
- Create: `pkg/events/task_events.go`
- Create: `internal/orchestration/state_machine.go`
- Test: `internal/orchestration/state_machine_test.go`

**Step 1: Write the failing test**

Create:

```go
package orchestration

import "testing"

func TestTaskStateMachine_AllowsHappyPathTransitions(t *testing.T) {
	sm := NewTaskStateMachine()

	state := Pending
	var err error

	state, err = sm.Transition(state, Planning)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, err = sm.Transition(state, Queued)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, err = sm.Transition(state, Running)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, err = sm.Transition(state, Evaluating)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = sm.Transition(state, Succeeded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestration -run TestTaskStateMachine_AllowsHappyPathTransitions -v`  
Expected: FAIL with undefined symbols such as `NewTaskStateMachine` or `Pending`

**Step 3: Write minimal implementation**

Implement:

```go
package orchestration

type TaskState string

const (
	Pending    TaskState = "pending"
	Planning   TaskState = "planning"
	Queued     TaskState = "queued"
	Running    TaskState = "running"
	Evaluating TaskState = "evaluating"
	Succeeded  TaskState = "succeeded"
	Failed     TaskState = "failed"
)

type TaskStateMachine struct{}

func NewTaskStateMachine() TaskStateMachine { return TaskStateMachine{} }
```

Add a small transition table and domain models for `Task`, `Plan`, `Action`, and task lifecycle events.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestration -run TestTaskStateMachine_AllowsHappyPathTransitions -v`  
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/taskdsl/task.go pkg/taskdsl/plan.go pkg/taskdsl/action.go pkg/events/task_events.go internal/orchestration/state_machine.go internal/orchestration/state_machine_test.go
git commit -m "feat: add task domain and state machine"
```

### Task 3: Define protobuf contracts for controller-worker communication

**Files:**
- Create: `api/proto/agentos/v1/task.proto`
- Create: `api/proto/agentos/v1/runtime.proto`
- Create: `api/README.md`
- Test: `api/proto/agentos/v1/proto_contract_test.go`

**Step 1: Write the failing test**

Create a lightweight Go test that validates required strings exist in the proto files:

```go
package v1

import (
	"os"
	"strings"
	"testing"
)

func TestTaskProto_DefinesExecuteActionRequest(t *testing.T) {
	data, err := os.ReadFile("task.proto")
	if err != nil {
		t.Fatalf("read proto: %v", err)
	}

	if !strings.Contains(string(data), "message ExecuteActionRequest") {
		t.Fatalf("missing ExecuteActionRequest contract")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./api/proto/agentos/v1 -run TestTaskProto_DefinesExecuteActionRequest -v`  
Expected: FAIL because proto files do not exist yet

**Step 3: Write minimal implementation**

Define protobuf contracts for:
- `Task`
- `Plan`
- `Action`
- `ExecuteActionRequest`
- `ExecuteActionResponse`
- `StreamChunk`
- `ResourceUsage`
- `RuntimeService`

Example skeleton:

```proto
syntax = "proto3";

package agentos.v1;

message ExecuteActionRequest {
  string task_id = 1;
  string action_id = 2;
  string runtime_profile = 3;
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./api/proto/agentos/v1 -run TestTaskProto_DefinesExecuteActionRequest -v`  
Expected: PASS

**Step 5: Commit**

```bash
git add api/proto/agentos/v1/task.proto api/proto/agentos/v1/runtime.proto api/README.md api/proto/agentos/v1/proto_contract_test.go
git commit -m "feat: define controller runtime protobuf contracts"
```

### Task 4: Build the Go control-plane skeleton

**Files:**
- Create: `cmd/controller/main.go`
- Create: `cmd/apiserver/main.go`
- Create: `cmd/osctl/main.go`
- Create: `internal/access/http/server.go`
- Create: `internal/access/cli/root.go`
- Create: `internal/orchestration/planner.go`
- Create: `internal/orchestration/task_engine.go`
- Create: `internal/messaging/event_bus.go`
- Create: `internal/persistence/task_repository.go`
- Create: `internal/runtimeclient/client.go`
- Test: `internal/orchestration/task_engine_test.go`

**Step 1: Write the failing test**

Create:

```go
package orchestration

import "testing"

func TestTaskEngine_StartTaskPublishesTaskCreatedEvent(t *testing.T) {
	repo := NewInMemoryTaskRepository()
	bus := NewInMemoryEventBus()
	engine := NewTaskEngine(repo, bus, StubPlanner{})

	task, err := engine.StartTask("write a sui contract")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.State != Planning {
		t.Fatalf("expected planning, got %s", task.State)
	}

	if len(bus.Events()) == 0 {
		t.Fatal("expected published events")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestration -run TestTaskEngine_StartTaskPublishesTaskCreatedEvent -v`  
Expected: FAIL because repository, bus, planner, and engine types are missing

**Step 3: Write minimal implementation**

Implement:
- `TaskEngine` with `StartTask(prompt string)` method
- in-memory repository and event bus for tests
- stub planner that returns one action such as `command.exec`
- thin HTTP server with `POST /v1/tasks`
- thin CLI command `osctl submit "<prompt>"`

Minimal task engine shape:

```go
type Planner interface {
	Plan(prompt string) (taskdsl.Plan, error)
}

type EventBus interface {
	Publish(topic string, payload any) error
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestration -run TestTaskEngine_StartTaskPublishesTaskCreatedEvent -v`  
Expected: PASS

Then run: `go test ./...`  
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/controller/main.go cmd/apiserver/main.go cmd/osctl/main.go internal/access/http/server.go internal/access/cli/root.go internal/orchestration/planner.go internal/orchestration/task_engine.go internal/messaging/event_bus.go internal/persistence/task_repository.go internal/runtimeclient/client.go internal/orchestration/task_engine_test.go
git commit -m "feat: add go control plane skeleton"
```

### Task 5: Build the Rust runtime skeleton

**Files:**
- Create: `runtime/crates/worker/Cargo.toml`
- Create: `runtime/crates/worker/src/main.rs`
- Create: `runtime/crates/worker/src/service.rs`
- Create: `runtime/crates/sandbox/Cargo.toml`
- Create: `runtime/crates/sandbox/src/lib.rs`
- Create: `runtime/crates/telemetry/Cargo.toml`
- Create: `runtime/crates/telemetry/src/lib.rs`
- Test: `runtime/crates/sandbox/tests/docker_provider.rs`

**Step 1: Write the failing test**

Create:

```rust
use sandbox::{DockerProvider, IsolationProvider, SandboxSpec};

#[tokio::test]
async fn docker_provider_returns_started_handle() {
    let provider = DockerProvider::new();
    let handle = provider.start(SandboxSpec::default()).await.expect("start");
    assert_eq!(handle.status, "started");
}
```

**Step 2: Run test to verify it fails**

Run: `cargo test -p sandbox docker_provider_returns_started_handle`  
Working directory: `runtime`  
Expected: FAIL because crate and symbols do not exist yet

**Step 3: Write minimal implementation**

Implement runtime abstractions:

```rust
pub trait IsolationProvider {
    async fn start(&self, spec: SandboxSpec) -> Result<SandboxHandle, SandboxError>;
    async fn stop(&self, id: &str) -> Result<(), SandboxError>;
}
```

MVP behavior:
- `DockerProvider` returns a stubbed `SandboxHandle`
- `worker` exposes a Tonic gRPC service matching `RuntimeService`
- `telemetry` crate defines stream item structs for stdout/stderr/resource usage

**Step 4: Run test to verify it passes**

Run: `cargo test -p sandbox docker_provider_returns_started_handle`  
Expected: PASS

Then run: `cargo test --workspace`  
Working directory: `runtime`  
Expected: PASS

**Step 5: Commit**

```bash
git add runtime/Cargo.toml runtime/crates/worker/Cargo.toml runtime/crates/worker/src/main.rs runtime/crates/worker/src/service.rs runtime/crates/sandbox/Cargo.toml runtime/crates/sandbox/src/lib.rs runtime/crates/sandbox/tests/docker_provider.rs runtime/crates/telemetry/Cargo.toml runtime/crates/telemetry/src/lib.rs
git commit -m "feat: add rust runtime skeleton"
```

### Task 6: Connect the first happy-path control-plane to runtime flow

**Files:**
- Create: `internal/runtimeclient/grpc_client.go`
- Create: `internal/messaging/nats_bus.go`
- Create: `deploy/docker-compose.yml`
- Create: `deploy/nats/nats.conf`
- Create: `deploy/postgres/init.sql`
- Create: `examples/basic-task/README.md`
- Test: `tests/integration/happy_path_test.go`

**Step 1: Write the failing test**

Create:

```go
package integration

import "testing"

func TestHappyPath_SubmitTaskToCompleted(t *testing.T) {
	t.Skip("enable after docker compose services are wired")
}
```

Immediately replace the skip with a real assertion contract:

```go
func TestHappyPath_SubmitTaskToCompleted(t *testing.T) {
	status := runHappyPathScenario(t)
	if status != "succeeded" {
		t.Fatalf("expected succeeded, got %s", status)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./tests/integration -run TestHappyPath_SubmitTaskToCompleted -v`  
Expected: FAIL because services and helper are not implemented

**Step 3: Write minimal implementation**

Implement:
- NATS-backed event bus adapter
- gRPC runtime client that calls Rust worker
- Docker Compose for NATS + PostgreSQL + controller + worker
- example scenario that submits a single task with one `command.exec` action

Keep the happy path narrow:
- one worker
- one controller
- one planner stub
- one action
- one successful completion

**Step 4: Run test to verify it passes**

Run: `go test ./tests/integration -run TestHappyPath_SubmitTaskToCompleted -v`  
Expected: PASS

Then run:
- `go test ./...`
- `cargo test --workspace`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/runtimeclient/grpc_client.go internal/messaging/nats_bus.go deploy/docker-compose.yml deploy/nats/nats.conf deploy/postgres/init.sql examples/basic-task/README.md tests/integration/happy_path_test.go
git commit -m "feat: wire first end to end agentos happy path"
```

### Task 7: Add developer workflow and contribution guardrails

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `CONTRIBUTING.md`
- Create: `docs/architecture/protocol-versioning.md`
- Create: `docs/architecture/runtime-security-notes.md`

**Step 1: Write the failing test**

Add a CI lint target to `Makefile`:

```make
.PHONY: verify-ci

verify-ci:
	pwsh -NoProfile -Command "if (!(Test-Path .github/workflows/ci.yml)) { exit 1 }"
```

**Step 2: Run test to verify it fails**

Run: `make verify-ci`  
Expected: FAIL because CI workflow file does not exist

**Step 3: Write minimal implementation**

Create:
- CI workflow that runs `go test ./...` and `cargo test --workspace`
- contribution guide with repository layout and local startup commands
- protocol versioning note for protobuf compatibility
- runtime security note documenting MVP Docker limitations and the future gVisor/Firecracker path

**Step 4: Run test to verify it passes**

Run: `make verify-ci`  
Expected: PASS

Then run:
- `go test ./...`
- `cargo test --workspace`

Expected: PASS

**Step 5: Commit**

```bash
git add .github/workflows/ci.yml CONTRIBUTING.md docs/architecture/protocol-versioning.md docs/architecture/runtime-security-notes.md Makefile
git commit -m "docs: add contributor and ci guardrails"
```

## Notes for execution

- Execute tasks in order; do not skip ahead to runtime or NATS before the Go domain model exists.
- Keep MVP scope narrow. Do not add memory, browser automation, OAuth, Web3 signing, or Firecracker in this plan.
- For any production code in a task, follow `@superpowers/test-driven-development` before implementation.
- Before starting implementation, create an isolated workspace with `@superpowers/using-git-worktrees`.
- After the final task, use `@superpowers/finishing-a-development-branch`.
