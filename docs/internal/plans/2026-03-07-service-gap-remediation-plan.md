# Service Gap Remediation Plan

## Goal

Repair the highest-value service gaps found in the repository review so the project has a working critical path for local execution, basic agent-facing HTTP wiring, and more accurate operator guidance.

## Scope For This Batch

### In scope

- Fix the broken local execution path when a direct Rust worker address is configured.
- Normalize the Go-to-Rust execution payload contract.
- Expose the existing agent list capability through HTTP and wire the gateway to the loaded agent manager.
- Update README / CI for the repaired path.
- Add regression tests for each repaired area.

### Out of scope

- Full multi-process distributed scheduling between `apiserver` and `controller`
- Multi-tenant auth / RBAC implementation
- UI / SDK work
- Persistent credential vault

## Agent Team

### Squad A — Runtime Path

**Mission:** Make the local control-plane-to-worker execution path actually work.

**Files likely touched:**

- `internal/orchestration/engine_impl.go`
- `internal/orchestration/engine_impl_test.go`
- `internal/adapters/runtimeclient/grpc.go`
- `runtime/crates/worker/src/grpc.rs` only if contract handling on the Rust side is needed

**Acceptance criteria:**

- When scheduler dispatch cannot find a worker but a direct executor exists, task execution still succeeds via the direct executor path.
- Go action payloads are accepted by the Rust worker without key-name mismatch.
- Regression tests cover the fallback path and payload normalization.

### Squad B — Gateway / Agent Surface

**Mission:** Make the agent-facing HTTP surface match the code that already exists.

**Files likely touched:**

- `cmd/apiserver/main.go`
- `internal/access/http/server.go`
- `internal/gateway/http.go`
- `internal/gateway/http_test.go`

**Acceptance criteria:**

- `apiserver` injects the loaded `AgentManager` into the gateway.
- `/agent/list` is exposed and returns loaded agent names.
- `/agent/run` uses the configured manager for agent existence checks.
- HTTP tests cover the new route and manager wiring behavior.

### Squad C — Docs / CI

**Mission:** Align repo guidance and automation with the repaired path.

**Files likely touched:**

- `README.md`
- `README_CN.md`
- `.github/workflows/ci.yml`

**Acceptance criteria:**

- Quick start text matches actual supported local flow.
- CI Go version matches the module declaration.
- Docs explain the difference between direct worker execution and controller-based worker registration.

## Execution Order

1. Add failing tests for runtime fallback and gateway surface.
2. Implement runtime fallback and payload normalization.
3. Implement gateway wiring and route exposure.
4. Update README / README_CN / CI.
5. Run `go test ./...`.
6. Run `cargo test --workspace`.

## Verification Commands

```bash
go test ./...
cd runtime && cargo test --workspace
```
