# Worker Availability Filter API Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `available_only=true` filtering to `GET /v1/workers` so operator tooling can request only schedulable workers directly from the API.

**Architecture:** Keep the existing read-only worker summary endpoint, but extend it with a query flag that switches from `Registry.List(...)` to `Registry.GetAvailable(...)`. Reuse the same response shape and summary counters so existing consumers do not need a new schema. Reject malformed query values with `400`.

**Tech Stack:** Go HTTP server, worker registry, Go unit tests, markdown API docs.

---

### Task 1: Team Alpha - Define failing filter behavior

**Files:**
- Modify: `internal/access/http/server_test.go`

**Step 1: Write the failing tests**
- Add a test that `GET /v1/workers?available_only=true` returns only schedulable workers.
- Add a test that malformed `available_only` values return `400`.

**Step 2: Run test to verify it fails**
Run: `go test ./internal/access/http -run 'Workers' -count=1`
Expected: FAIL because the endpoint ignores the filter today.

### Task 2: Team Beta - Implement registry-backed filtering

**Files:**
- Modify: `internal/access/http/server.go`
- Modify: `internal/access/http/server_test.go` (stub support only)

**Step 1: Write minimal implementation**
- Extend the HTTP-facing registry interface to support `GetAvailable(ctx)`.
- Parse `available_only` from the query string.
- Route `true` to `GetAvailable`, `false` or empty to `List`.
- Return `400` for malformed values.

**Step 2: Run test to verify it passes**
Run: `go test ./internal/access/http -run 'Workers' -count=1`
Expected: PASS

### Task 3: Team Gamma - Update docs

**Files:**
- Modify: `docs/reference/api-surfaces.md`
- Modify: `docs/reference/api-surfaces-cn.md`

**Step 1: Update docs**
- Document the new `available_only` query flag for `GET /v1/workers`.
- Clarify that `summary` applies to the returned result set.

**Step 2: Smoke-check docs references**
Run: `rg -n "available_only|/v1/workers" docs/reference/api-surfaces*.md`
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
