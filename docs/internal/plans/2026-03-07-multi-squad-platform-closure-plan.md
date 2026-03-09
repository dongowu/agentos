# Multi-Squad Platform Closure Plan

## Goal

Close the highest-priority platform gaps across shared worker registration, orchestration context propagation, memory integration, credential token injection, and better default planning behavior.

## Squads

### Squad A — Shared Registry Loop (P0)

**Objective:** Make `apiserver -> controller -> worker` use a shared worker registry and a functioning scheduler dispatch path.

**Deliverables:**

- Extend WorkerRegistry gRPC with worker listing support.
- Add a Go remote registry client implementing `worker.Registry`.
- Add a real gRPC dialer for `worker.Pool` in bootstrap.
- Bootstrap `apiserver` against a remote control plane registry when configured.

### Squad B — Orchestration Context (P1)

**Objective:** Carry `AgentName` and `TenantID` into orchestration so policy can evaluate real agent context.

**Deliverables:**

- Add richer task-start input to orchestration without breaking existing callers.
- Pass agent context from HTTP gateway and task API into engine.
- Use `AgentName` / `TenantID` in policy evaluation.

### Squad C — Memory + Vault (P1)

**Objective:** Connect the already-implemented subsystems into the orchestration path.

**Deliverables:**

- Recall relevant memory before planning.
- Store action execution results into memory after completion.
- Load agent secrets into the in-memory vault from config/env.
- Inject opaque credential tokens into action payload env before execution.

### Squad D — Default Planner (P2)

**Objective:** Replace the misleading `echo ok` dev fallback with a more honest default planner.

**Deliverables:**

- Add a prompt-driven fallback planner that uses the original prompt.
- Keep LLM planner as the preferred path when configured.

## Test Strategy

1. Add failing tests for each squad first.
2. Fix only what the tests prove.
3. Run targeted package tests.
4. Run full `go test ./...`.
5. Run full `cargo test --workspace`.

## Verification Commands

```bash
go test ./...
cd runtime && cargo test --workspace
```
