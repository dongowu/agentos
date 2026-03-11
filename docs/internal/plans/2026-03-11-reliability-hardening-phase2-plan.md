# Reliability Hardening Phase 2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Push AgentOS reliability further by replacing string-matched scheduler failures with typed errors, adding stale-task recovery on startup, and extending multiprocess acceptance to cover restart/chaos scenarios.

**Architecture:** This batch is organized as an agent-team style effort with one lead and three implementation teams. Team Alpha owns scheduler error contracts and retry classification. Team Beta owns recoverability of queued/running tasks after process interruption. Team Gamma owns end-to-end acceptance and restart scenario coverage that validates Alpha and Beta together without expanding product scope.

**Tech Stack:** Go control plane, Rust worker runtime, in-memory/Postgres persistence adapters, bash acceptance scripts, Go tests, Rust tests.

## Current Status

- Team Alpha: completed on 2026-03-11
- Team Beta: completed on 2026-03-11
- Team Gamma: completed on 2026-03-11
- Lead integration verification: Go + Rust test suites passing; `./scripts/acceptance.sh` passing with transient no-worker + worker restart coverage

---

## Team Topology

- **Lead Team** — integration owner, verification owner
- **Team Alpha** — scheduler contracts, retry policy, orchestration wiring
- **Team Beta** — persistence query surfaces, stale-task sweeper, startup recovery
- **Team Gamma** — `scripts/acceptance.sh`, restart scenarios, docs for operational behavior

## Dependency Graph

```text
Team Alpha ─┐
            ├─→ Lead integration + full verification
Team Beta ──┤
            └─→ Team Gamma final acceptance assertions
```

- Team Alpha and Team Beta can start in parallel.
- Team Gamma can start immediately on acceptance harness structure and failing restart scenarios, but its final green pass depends on Alpha/Beta landing first.

### Task 1: Team Alpha - Typed scheduler errors and configurable retries

**Files:**
- Modify: `internal/scheduler/local_scheduler.go`
- Modify: `internal/scheduler/nats_dispatcher.go`
- Modify: `internal/worker/pool.go`
- Modify: `internal/orchestration/engine_impl.go`
- Modify: `internal/orchestration/engine_impl_test.go`
- Modify: `pkg/config/config.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `docs/reference/configuration.md`
- Modify: `docs/reference/configuration-cn.md`

**Objective:**
Replace string parsing of scheduler failures with typed errors, then make retry policy configurable without changing default behavior too broadly.

**Acceptance Criteria:**
- Scheduler/worker selection returns a stable typed error for "no available workers".
- Orchestration retry logic uses `errors.Is` or equivalent typed matching, not message matching.
- Retry count and backoff become config-driven with safe defaults.
- Existing retry tests are updated and still pass.

**Step 1: Write the failing tests**

- Add Go tests proving orchestration retries when `worker.ErrNoAvailableWorkers` is returned.
- Add Go tests proving non-retryable scheduler failures still fail fast.
- Add config tests proving retry knobs parse from config/env correctly.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/orchestration ./internal/worker ./pkg/config -run 'Retry|NoAvailable|Scheduler' -count=1`
Expected: FAIL because retry classification is still string-based and config knobs do not exist.

**Step 3: Write minimal implementation**

- Export a typed sentinel error from the worker/scheduler boundary.
- Thread that error through local and NATS scheduling paths.
- Replace `strings.Contains` retry detection with typed matching.
- Add `scheduler.submit_retries` and `scheduler.submit_retry_backoff` config support.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/orchestration ./internal/worker ./pkg/config -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/scheduler/local_scheduler.go internal/scheduler/nats_dispatcher.go internal/worker/pool.go internal/orchestration/engine_impl.go internal/orchestration/engine_impl_test.go pkg/config/config.go internal/bootstrap/bootstrap.go docs/reference/configuration.md docs/reference/configuration-cn.md
git commit -m "feat: add typed scheduler retry contracts"
```

### Task 2: Team Beta - Stale task sweeper and startup recovery

**Files:**
- Modify: `internal/persistence/task_repository.go`
- Modify: `internal/adapters/persistence/memory/memory.go`
- Modify: `internal/adapters/persistence/postgres/postgres.go`
- Modify: `internal/orchestration/contracts.go`
- Modify: `internal/orchestration/engine_impl.go`
- Create: `internal/orchestration/recovery.go`
- Create: `internal/orchestration/recovery_test.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `pkg/config/config.go`
- Modify: `docs/reference/configuration.md`
- Modify: `docs/reference/configuration-cn.md`

**Objective:**
Recover tasks left in `queued` or `running` state when the service restarts, and mark truly stale executions deterministically instead of leaving them stranded forever.

**Acceptance Criteria:**
- Persistence layer can list recoverable tasks by state.
- Startup recovery sweeper can requeue `queued` tasks and fail/mark stale `running` tasks according to policy.
- Recovery behavior is configurable and covered in memory + postgres tests.
- Recovery does not affect terminal tasks.

**Step 1: Write the failing tests**

- Add repository tests proving recoverable tasks can be listed.
- Add orchestration recovery tests for:
  - queued task -> requeued/runnable
  - stale running task -> failed with explicit recovery reason
  - succeeded/failed tasks -> untouched

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapters/persistence/... ./internal/orchestration -run 'Recover|Recovery|Recoverable' -count=1`
Expected: FAIL because task listing/recovery primitives do not exist.

**Step 3: Write minimal implementation**

- Extend `TaskRepository` with a recoverable-task listing method.
- Implement listing in both memory and Postgres adapters.
- Add a recovery runner that executes at bootstrap before serving traffic.
- Add config knobs for recovery enablement and stale-running timeout.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapters/persistence/... ./internal/orchestration ./internal/bootstrap -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/persistence/task_repository.go internal/adapters/persistence/memory/memory.go internal/adapters/persistence/postgres/postgres.go internal/orchestration/contracts.go internal/orchestration/engine_impl.go internal/orchestration/recovery.go internal/orchestration/recovery_test.go internal/bootstrap/bootstrap.go pkg/config/config.go docs/reference/configuration.md docs/reference/configuration-cn.md
git commit -m "feat: add startup task recovery sweeper"
```

### Task 3: Team Gamma - Multiprocess acceptance and restart/chaos coverage

**Files:**
- Modify: `scripts/acceptance.sh`
- Modify: `docs/architecture/multiprocess-acceptance.md`
- Modify: `docs/reference/runtime-and-sandbox.md`
- Modify: `docs/reference/runtime-and-sandbox-cn.md`
- Optionally create: `scripts/acceptance-restart.sh` (only if script split is clearly cleaner than extending the existing script)

**Objective:**
Extend the real multiprocess acceptance path so reliability behaviors are exercised end-to-end: worker re-registration, transient no-worker windows, and startup recovery visibility.

**Acceptance Criteria:**
- Acceptance covers worker restart/controller reconnect behavior.
- Acceptance covers a temporary no-worker window without immediate task loss.
- Acceptance documents the recovery expectations and operational signals.
- Script remains deterministic and developer-runnable locally.

**Step 1: Write the failing scenario assertions**

- Add script assertions for:
  - worker kill/restart -> worker re-registers and subsequent task still succeeds
  - transient worker absence -> submit path survives when retries are enabled
  - recovery behavior logging or observable task-state outcome after restart

**Step 2: Run script to verify it fails**

Run: `./scripts/acceptance.sh`
Expected: FAIL on the new restart/recovery assertions until Alpha/Beta land.

**Step 3: Write minimal implementation**

- Extend the acceptance harness with restart helpers and explicit waits.
- Prefer environment flags over hardcoded behavior so scenarios can be toggled.
- Update docs to match the new operational guarantees.

**Step 4: Run script to verify it passes**

Run: `./scripts/acceptance.sh`
Expected: PASS

**Step 5: Commit**

```bash
git add scripts/acceptance.sh docs/architecture/multiprocess-acceptance.md docs/reference/runtime-and-sandbox.md docs/reference/runtime-and-sandbox-cn.md
git commit -m "test: extend multiprocess acceptance for restart recovery"
```

### Task 4: Lead Team - Integration and verification

**Files:**
- Verify only

**Step 1: Run focused verification**

Run: `go test ./internal/orchestration ./internal/worker ./internal/adapters/persistence/... ./internal/bootstrap -count=1`
Expected: PASS

Run: `cargo test --workspace -q`
Expected: PASS

**Step 2: Run end-to-end verification**

Run: `go test ./... -count=1`
Expected: PASS

Run: `./scripts/acceptance.sh`
Expected: PASS

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: verify reliability hardening phase 2"
```
