# Worker Summary API Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an authenticated `GET /v1/workers` endpoint so operators and future tooling can inspect the current worker registry directly instead of inferring capacity only from health snapshots.

**Architecture:** Reuse the existing `worker.Registry` already injected into the HTTP server. Expose a read-only REST resource at `/v1/workers` that returns both per-worker snapshots and a small aggregate summary, with authentication matching the existing `/v1/*` rules. Keep the first version minimal: no pagination, no mutations, no new storage.

**Tech Stack:** Go HTTP server, worker registry, Go unit tests, markdown API docs.

---

### Task 1: Team Alpha - Define failing worker API behavior

**Files:**
- Modify: `internal/access/http/server_test.go`

**Step 1: Write the failing test**
- Add a test that `GET /v1/workers` returns `200` with worker snapshots plus aggregate counts when auth succeeds.
- Add a test that the route returns `500` when the worker registry is unavailable.
- Add a test that the route follows the existing `/v1/*` bearer-token requirement.

**Step 2: Run test to verify it fails**
Run: `go test ./internal/access/http -run 'Workers' -count=1`
Expected: FAIL because `/v1/workers` is not registered yet.

### Task 2: Team Beta - Implement the minimal worker summary endpoint

**Files:**
- Modify: `internal/access/http/server.go`

**Step 1: Write minimal implementation**
- Register `GET /v1/workers`.
- Authenticate the request using the existing server auth flow.
- Read the worker registry, sort workers deterministically, and return a JSON response containing `workers` plus a `summary` block derived from the existing health counter helper.
- Return `500` when no worker registry is configured or when listing workers fails.

**Step 2: Run test to verify it passes**
Run: `go test ./internal/access/http -run 'Workers' -count=1`
Expected: PASS

### Task 3: Team Gamma - Document the new API surface

**Files:**
- Modify: `docs/reference/api-surfaces.md`
- Modify: `docs/reference/api-surfaces-cn.md`

**Step 1: Update docs**
- Add `GET /v1/workers` to the authenticated API table.
- Provide a response example that shows both worker details and summary counts.

**Step 2: Smoke-check docs references**
Run: `rg -n "/v1/workers|summary|available_workers" docs/reference/api-surfaces*.md`
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
