# Reliability Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve AgentOS reliability by hardening worker re-registration, adding task submission recovery, and tightening execution safety controls.

**Architecture:** This batch is split into three squads with clear ownership. Squad A hardens the control-plane/runtime handshake so workers survive controller restarts and duplicate registrations. Squad B improves orchestration recovery by retrying transient dispatch failures instead of failing tasks immediately. Squad C strengthens shell safety in the Rust runtime without changing the broader execution model.

**Tech Stack:** Go control plane, Rust worker/runtime, gRPC, PostgreSQL/memory persistence, Go tests, Rust unit tests.

---

### Task 1: Squad A - Worker registration and reconnection hardening

**Files:**
- Modify: `internal/worker/memory_registry.go`
- Modify: `internal/worker/registry_test.go`
- Modify: `internal/worker/grpc_server_test.go`
- Modify: `runtime/crates/worker/src/registration.rs`
- Test: `runtime/crates/worker/src/registration.rs`

**Step 1: Write the failing tests**

- Add a Go test proving a worker can re-register with the same `worker_id` and refresh its address/capacity instead of being permanently rejected.
- Add a Rust test proving the heartbeat loop attempts a re-registration after the control plane rejects a heartbeat.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/worker -run 'TestMemoryRegistry_RegisterDuplicate|TestRegisterDuplicate' -count=1`
Expected: FAIL because duplicate registrations are currently rejected.

Run: `cargo test -p agentos-worker heartbeat_loop -- --nocapture`
Expected: FAIL because the heartbeat loop currently only logs heartbeat errors.

**Step 3: Write minimal implementation**

- Change `MemoryRegistry.Register` to upsert an existing worker snapshot instead of rejecting duplicate IDs.
- Keep `LastHeartbeat` fresh and reset status to `online` on successful re-registration.
- Update the Rust heartbeat loop to attempt `register()` when `heartbeat()` returns a rejection or transport error.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/worker -count=1`
Expected: PASS

Run: `cargo test -p agentos-worker registration -- --nocapture`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worker/memory_registry.go internal/worker/registry_test.go internal/worker/grpc_server_test.go runtime/crates/worker/src/registration.rs
git commit -m "feat: harden worker registration recovery"
```

### Task 2: Squad B - Task submission recovery for transient scheduler failures

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `internal/orchestration/engine_impl.go`
- Modify: `internal/orchestration/engine_impl_test.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `docs/reference/configuration.md`
- Modify: `docs/reference/configuration-cn.md`

**Step 1: Write the failing tests**

- Add a Go test proving task submission retries when the scheduler reports a transient `no available workers` failure before eventually succeeding.
- Add a Go test proving the next action in `ProcessResults` also retries before failing the task.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/orchestration -run 'TestEngineImpl_.*Retry' -count=1`
Expected: FAIL because scheduler submission currently fails fast unless a direct executor exists.

**Step 3: Write minimal implementation**

- Add scheduler recovery config (`submit_retries`, `submit_retry_backoff`) with conservative defaults.
- Centralize scheduler submission in a helper that retries transient worker-unavailable errors.
- Reuse that helper from both initial `StartTaskWithInput` dispatch and follow-up dispatch inside `ProcessResults`.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/orchestration -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/config/config.go internal/orchestration/engine_impl.go internal/orchestration/engine_impl_test.go internal/bootstrap/bootstrap.go docs/reference/configuration.md docs/reference/configuration-cn.md
git commit -m "feat: retry transient scheduler submission failures"
```

### Task 3: Squad C - Shell execution safety hardening

**Files:**
- Modify: `runtime/crates/sandbox/src/security.rs`
- Test: `runtime/crates/sandbox/src/security.rs`
- Modify: `docs/reference/runtime-and-sandbox.md`
- Modify: `docs/reference/runtime-and-sandbox-cn.md`

**Step 1: Write the failing tests**

- Add Rust tests proving supervised/semi-autonomous modes reject compound shell commands like `echo ok && rm -rf /tmp`.
- Add Rust tests proving env-style secret assignments like `API_KEY=...` are redacted.

**Step 2: Run tests to verify they fail**

Run: `cargo test -p agentos-sandbox security -- --nocapture`
Expected: FAIL because compound commands can currently bypass the simple base-command check and env-style secrets are not fully redacted.

**Step 3: Write minimal implementation**

- Detect shell control operators (`&&`, `||`, `;`, pipes, subshell markers) and block them in supervised/semi-autonomous modes unless explicitly whitelisted by a full pattern match.
- Extend secret redaction patterns to cover `NAME=value` assignments for sensitive variable names.

**Step 4: Run tests to verify they pass**

Run: `cargo test -p agentos-sandbox -q`
Expected: PASS

**Step 5: Commit**

```bash
git add runtime/crates/sandbox/src/security.rs docs/reference/runtime-and-sandbox.md docs/reference/runtime-and-sandbox-cn.md
git commit -m "feat: harden shell execution safety rules"
```

### Task 4: Integration verification

**Files:**
- Verify only

**Step 1: Run focused verification**

Run: `go test ./internal/worker ./internal/orchestration -count=1`
Expected: PASS

Run: `cargo test -p agentos-worker registration -q`
Expected: PASS

Run: `cargo test -p agentos-sandbox -q`
Expected: PASS

**Step 2: Run broader verification**

Run: `go test ./... -count=1`
Expected: PASS

Run: `cargo test --workspace -q`
Expected: PASS

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: verify reliability hardening batch"
```
