# CLI Readiness Diagnostics Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `claw dev` show both liveness and readiness so operators can immediately see when the service is up but temporarily unschedulable.

**Architecture:** Keep the existing diagnostics flow in `claw dev`, but add a second public probe call to `/ready`. Reuse the same lightweight JSON decoding path used for `/health`, and surface degraded readiness reasons in the CLI output without changing task submission behavior.

**Tech Stack:** Go CLI, Cobra commands, Go unit tests, markdown docs.

---

### Task 1: Team Alpha - Define failing CLI diagnostics behavior

**Files:**
- Modify: `cmd/claw-cli/main_test.go`

**Step 1: Write the failing tests**
- Add a test that `claw dev` prints both `server` and `readiness` lines when `/health` and `/ready` are both healthy.
- Add a test that degraded readiness reasons are shown when `/ready` returns `503` with a degraded payload.
- Extend bearer-token diagnostics coverage to assert `/ready` also receives the auth header.

**Step 2: Run tests to verify they fail**
Run: `go test ./cmd/claw-cli -run 'DevCmd' -count=1`
Expected: FAIL because diagnostics currently call only `/health` and do not print readiness.

### Task 2: Team Beta - Implement readiness-aware diagnostics

**Files:**
- Modify: `cmd/claw-cli/main.go`

**Step 1: Write minimal implementation**
- Add a shared helper for public probe endpoints.
- Introduce `fetchReady()`.
- Update `claw dev` diagnostics output to print liveness, readiness, and degraded reasons when present.

**Step 2: Run tests to verify they pass**
Run: `go test ./cmd/claw-cli -run 'DevCmd' -count=1`
Expected: PASS

### Task 3: Team Gamma - Document the diagnostics improvement

**Files:**
- Modify: `docs/concepts/clawos-v1-architecture.md`

**Step 1: Update docs**
- Note that `claw dev` now checks both liveness and readiness.

**Step 2: Smoke-check docs references**
Run: `rg -n "claw dev|readiness|/ready" docs/concepts/clawos-v1-architecture.md`
Expected: matches for the updated diagnostics description.

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
