# Health Observability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Upgrade the current `/health` endpoint from a liveness-only check into an operator-useful runtime summary for scheduler and worker capacity.

**Architecture:** Keep the existing `/health` route, but enrich its JSON payload with scheduler mode, recovery configuration, and worker-capacity counters derived from the existing worker registry. Report `degraded` when the control plane is up but there are currently no schedulable workers, without introducing a new admin API surface.

**Tech Stack:** Go HTTP server, bootstrap wiring, worker registry, Go unit tests, markdown reference docs.

---

### Task 1: Team Health - Define failing /health behavior

**Files:**
- Modify: `internal/access/http/server_test.go`
- Modify: `internal/access/http/auth_test.go` (only if auth interactions need coverage)

**Step 1: Write the failing tests**
- Add a test that `/health` returns scheduler and worker counters when a registry is attached.
- Add a test that `/health` returns `degraded` when there are zero currently available workers.
- Keep backward compatibility by asserting the route still works without auth.

**Step 2: Run tests to verify they fail**
Run: `go test ./internal/access/http -run 'Health' -count=1`
Expected: FAIL because `/health` currently returns only `{"status":"ok"}`.

### Task 2: Team Wiring - Implement health snapshot and apiserver wiring

**Files:**
- Modify: `internal/access/http/server.go`
- Modify: `cmd/apiserver/main.go`

**Step 1: Write minimal implementation**
- Add a small optional health-registry interface to the HTTP server.
- Compute worker totals and available capacity from `worker.Registry.List(...)`.
- Include scheduler mode and recovery enabled state in the health response.
- Mark the status as `degraded` when registry-backed scheduling has no available worker slots.

**Step 2: Run tests to verify they pass**
Run: `go test ./internal/access/http -run 'Health' -count=1`
Expected: PASS

### Task 3: Team Docs - Document the richer health surface

**Files:**
- Modify: `docs/reference/api-surfaces.md`
- Modify: `docs/reference/api-surfaces-cn.md`

**Step 1: Update docs**
- Replace the liveness-only `/health` example with the richer response shape.
- Document what `degraded` means operationally.

**Step 2: Smoke-check docs references**
Run: `rg -n "degraded|scheduler_mode|available_workers" docs/reference/api-surfaces*.md`
Expected: matches in both docs.

### Task 4: Lead Team - Verify the service optimization

**Files:**
- Verify only

**Step 1: Run focused verification**
Run: `go test ./internal/access/http ./cmd/apiserver -count=1`
Expected: PASS

**Step 2: Run full verification**
Run: `go test ./... -count=1`
Expected: PASS

Run: `cargo test --workspace -q`
Expected: PASS
