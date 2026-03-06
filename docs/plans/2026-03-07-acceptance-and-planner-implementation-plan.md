# Acceptance And Planner Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a real multi-process acceptance flow for controller/apiserver/worker and strengthen planner defaults with safer LLM auto-enablement.

**Architecture:** Add a repo-owned acceptance script that boots the real binaries as separate processes, waits for registration/health, submits a task, and verifies end-to-end success. In parallel, upgrade the default prompt planner from pass-through fallback to heuristic multi-action planning and make bootstrap automatically use the LLM planner only when valid OpenAI-compatible config is present.

**Tech Stack:** Go, Rust, bash, gRPC, HTTP, existing OpenAI-compatible LLM adapter.

---

### Task 1: Acceptance surface hardening

**Files:**
- Modify: `cmd/apiserver/main.go`
- Modify: `internal/access/http/server.go`
- Test: `internal/access/http/server_test.go`
- Test: `cmd/apiserver/main_test.go`

**Step 1: Write failing tests**
- Add a server test asserting `GET /health` returns `200` and JSON status.
- Add an apiserver test asserting listen address can be overridden by env.

**Step 2: Run targeted tests to verify failure**
- Run: `GOCACHE=/tmp/go-build-agentos go test ./internal/access/http ./cmd/apiserver`

**Step 3: Write minimal implementation**
- Add `/health` route.
- Add a helper for configurable API listen address.

**Step 4: Re-run targeted tests**
- Run the same command and verify green.

### Task 2: Stronger default planner

**Files:**
- Modify: `internal/orchestration/prompt_planner.go`
- Test: `internal/orchestration/prompt_planner_test.go`

**Step 1: Write failing tests**
- Add tests for explicit `file.read`, `file.write`, `http.request`, and multi-step `then` decomposition.

**Step 2: Run targeted tests to verify failure**
- Run: `GOCACHE=/tmp/go-build-agentos go test ./internal/orchestration -run PromptPlanner`

**Step 3: Write minimal implementation**
- Parse explicit user intents heuristically and emit typed actions.
- Fall back to `command.exec` when no heuristic matches.

**Step 4: Re-run targeted tests**
- Run the same command and verify green.

### Task 3: LLM auto-enablement hardening

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Test: `pkg/config/config_test.go`
- Test: `internal/bootstrap/bootstrap_test.go`
- Modify: `runtime/crates/worker/src/registration.rs`

**Step 1: Write failing tests**
- Add config tests for env overrides of LLM provider/model/base URL/API key.
- Add bootstrap tests proving missing API key falls back to non-LLM planner.
- Add Rust registration tests proving control plane addresses without scheme are normalized.

**Step 2: Run targeted tests to verify failure**
- Run: `GOCACHE=/tmp/go-build-agentos go test ./pkg/config ./internal/bootstrap`
- Run: `cargo test -p agentos-worker registration`

**Step 3: Write minimal implementation**
- Support env overrides for LLM settings in all modes.
- Auto-enable OpenAI-compatible planner when API key exists.
- Fall back to heuristic planner when LLM config is incomplete.
- Normalize Rust worker control plane addresses to accept bare host:port.

**Step 4: Re-run targeted tests**
- Run the same commands and verify green.

### Task 4: Real multi-process acceptance

**Files:**
- Create: `scripts/acceptance.sh`
- Create: `docs/architecture/multiprocess-acceptance.md`

**Step 1: Write script-based acceptance flow**
- Build the real binaries.
- Start controller, worker, and apiserver as separate processes.
- Wait for worker registration and HTTP health.
- Submit a task through HTTP.
- Poll until the task reaches `succeeded`.
- Print log locations and fail loudly with logs on error.

**Step 2: Run acceptance**
- Run: `./scripts/acceptance.sh`

**Step 3: Document usage**
- Add a concise doc describing what the script proves and required envs.

### Task 5: Full verification

**Files:**
- None

**Step 1: Run full Go verification**
- Run: `GOCACHE=/tmp/go-build-agentos go test ./...`

**Step 2: Run full Rust verification**
- Run: `cargo test --workspace`

**Step 3: Run acceptance again**
- Run: `./scripts/acceptance.sh`
