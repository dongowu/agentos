# Claw Dev Worker Capacity Diagnostics Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend `claw dev` so it shows currently schedulable workers, not just liveness/readiness, making the default developer diagnostics path more actionable.

**Architecture:** Keep the existing `claw dev` flow (`/health`, `/ready`, `/agent/list`) and add one more public read call to `GET /v1/workers?available_only=true`. Reuse the same HTTP request helper already used by the command, and print a concise line summarizing available worker IDs or `(none)` when no worker is schedulable.

**Tech Stack:** Go CLI, Go HTTP requests, Go unit tests, markdown architecture docs.

---

### Task 1: Team Alpha - Define failing diagnostics behavior

**Files:**
- Modify: `cmd/claw-cli/main_test.go`

**Step 1: Write the failing tests**
- Add a test that `claw dev` prints the list of currently available worker IDs when `/v1/workers?available_only=true` returns workers.
- Add a test that `claw dev` prints `available workers: (none)` when the response set is empty.
- Extend bearer-token diagnostics coverage to assert `/v1/workers?available_only=true` also receives the auth header.

**Step 2: Run tests to verify they fail**
Run: `go test ./cmd/claw-cli -run 'DevCmd' -count=1`
Expected: FAIL because `claw dev` does not fetch or print worker capacity yet.

### Task 2: Team Beta - Implement worker-capacity diagnostics

**Files:**
- Modify: `cmd/claw-cli/main.go`

**Step 1: Write minimal implementation**
- Add a small response type and fetch helper for `/v1/workers?available_only=true`.
- Update `claw dev` output to print one extra line for available workers.
- Keep task submission mode unchanged.

**Step 2: Run tests to verify it passes**
Run: `go test ./cmd/claw-cli -run 'DevCmd' -count=1`
Expected: PASS

### Task 3: Team Gamma - Update docs

**Files:**
- Modify: `docs/concepts/clawos-v1-architecture.md`

**Step 1: Update docs**
- Note that `claw dev` now shows current schedulable workers in addition to liveness/readiness.

**Step 2: Smoke-check docs references**
Run: `rg -n "claw dev|schedulable workers|available workers" docs/concepts/clawos-v1-architecture.md`
Expected: matches for the updated diagnostics note.

### Task 4: Lead Team - Verify the optimization batch

**Files:**
- Verify only

**Step 1: Run focused verification**
Run: `go test ./cmd/claw-cli -count=1`
Expected: PASS

**Step 2: Run full verification**
Run: `go test ./... -count=1`
Expected: PASS

Run: `cargo test --workspace -q`
Expected: PASS
