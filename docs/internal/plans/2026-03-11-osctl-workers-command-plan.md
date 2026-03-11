# Osctl Workers Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `osctl workers` command so operators can inspect current worker capacity from the CLI using the new worker-summary HTTP API.

**Architecture:** Keep `osctl submit` and `osctl status` unchanged, and add a remote-only worker inspection path that calls `GET /v1/workers` with optional `available_only=true`. Reuse the existing HTTP client adapter in `internal/access/cli`, keep local embedded mode unsupported for this command, and print a concise text summary plus worker rows.

**Tech Stack:** Go CLI, Cobra, HTTP client adapter, Go unit tests, markdown architecture docs.

---

### Task 1: Team Alpha - Define failing CLI behavior

**Files:**
- Modify: `internal/access/cli/root_test.go`

**Step 1: Write the failing tests**
- Add a test that `osctl workers --available` uses the remote worker client and prints a summary plus worker rows.
- Add a test that `osctl workers` fails without `--server`.
- Add a test that the remote HTTP worker client sends `available_only=true` and the bearer token.

**Step 2: Run test to verify it fails**
Run: `go test ./internal/access/cli -run 'Workers|HTTPTaskAPI' -count=1`
Expected: FAIL because no worker command or remote client method exists yet.

### Task 2: Team Beta - Implement the worker command

**Files:**
- Modify: `internal/access/cli/root.go`
- Modify: `internal/access/cli/http_client.go`

**Step 1: Write minimal implementation**
- Extend the remote CLI adapter with a worker-list method.
- Add `osctl workers` with an `--available` flag.
- Require `--server` for this command and print summary/worker lines in a stable order.

**Step 2: Run test to verify it passes**
Run: `go test ./internal/access/cli -run 'Workers|HTTPTaskAPI' -count=1`
Expected: PASS

### Task 3: Team Gamma - Update docs

**Files:**
- Modify: `docs/concepts/agentos-v1-architecture.md`

**Step 1: Update docs**
- Add `osctl workers` to the CLI command list.
- Note that it inspects worker capacity through the control-plane API.

**Step 2: Smoke-check docs references**
Run: `rg -n "osctl workers|worker capacity" docs/concepts/agentos-v1-architecture.md`
Expected: matches for the new command documentation.

### Task 4: Lead Team - Verify the optimization batch

**Files:**
- Verify only

**Step 1: Run focused verification**
Run: `go test ./internal/access/cli -count=1`
Expected: PASS

**Step 2: Run full verification**
Run: `go test ./... -count=1`
Expected: PASS

Run: `cargo test --workspace -q`
Expected: PASS
