# Readiness Semantics Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Split the current operator-facing health surface into liveness (`/health`) and schedulability readiness (`/ready`) without regressing the richer worker summary added earlier.

**Architecture:** Keep `/health` as a public liveness endpoint that always answers `200` with the current control-plane summary, even when the platform is degraded. Add a new public `/ready` endpoint that reuses the same summary payload but maps degraded capacity states to HTTP `503`, so load balancers and operators can distinguish "server is up" from "server can currently accept work".

**Tech Stack:** Go HTTP server, Go unit tests, apiserver wiring, markdown reference docs.

---

### Task 1: Team Alpha - Define failing readiness behavior

**Files:**
- Modify: `internal/access/http/server_test.go`

**Step 1: Write the failing tests**
- Add a test that `GET /ready` returns `200` and the worker summary when at least one worker is schedulable.
- Add a test that `GET /ready` returns `503` with `status = degraded` when no workers are available.

**Step 2: Run tests to verify they fail**
Run: `go test ./internal/access/http -run 'Ready' -count=1`
Expected: FAIL because `/ready` does not exist yet.

### Task 2: Team Beta - Implement public readiness semantics

**Files:**
- Modify: `internal/access/http/server.go`

**Step 1: Write minimal implementation**
- Factor the current health summary building into a helper that can be shared by `/health` and `/ready`.
- Register a public `/ready` route.
- Return `200` from `/ready` only when the service is considered schedulable; otherwise return `503` with the same JSON payload.

**Step 2: Run tests to verify they pass**
Run: `go test ./internal/access/http -run 'Ready' -count=1`
Expected: PASS

### Task 3: Team Gamma - Update reference docs

**Files:**
- Modify: `docs/reference/api-surfaces.md`
- Modify: `docs/reference/api-surfaces-cn.md`

**Step 1: Update docs**
- Document both `/health` and `/ready`.
- Explain the HTTP status code difference and the degraded readiness behavior.

**Step 2: Smoke-check docs references**
Run: `rg -n "/ready|degraded|liveness|readiness" docs/reference/api-surfaces*.md`
Expected: matches in both docs.

### Task 4: Lead Team - Verify the optimization batch

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
