# Agent Team Parallel Roadmap Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Use an agent-team model to execute the three missing product directions in parallel, integrate them safely, and merge verified work into `main`.

**Architecture:** Keep the existing Go control plane and Rust runtime unchanged as the execution substrate. Add product-facing capability in three parallel tracks: developer platform, enterprise governance, and ecosystem/integrations. Each track works in its own git worktree and branch, merges into a shared integration branch first, then lands in `main` only after end-to-end verification.

**Tech Stack:** Go control plane, Rust worker runtime, HTTP/SSE APIs, NATS/Postgres adapters, git worktrees, parallel subagents.

---

## Parallel Delivery Model

- Track A: `developer-platform` - productization for R&D efficiency teams
- Track B: `enterprise-governance` - permissions, tenancy, approval, governance
- Track C: `ecosystem-integrations` - templates, SDK surfaces, external integrations
- Integration branch: `integration/agent-team-phase-1`
- Final target: `main`

## Branch And Worktree Strategy

- Worktree root: prefer `.worktrees/` if it exists and is gitignored; otherwise `worktrees/`; otherwise create `.worktrees/` and add it to `.gitignore`
- Branches:
  - `feat/developer-platform-phase-1`
  - `feat/enterprise-governance-phase-1`
  - `feat/ecosystem-integrations-phase-1`
  - `integration/agent-team-phase-1`
- Rule: no track merges directly to `main`
- Rule: each track must pass its own focused tests before integration
- Rule: integration branch must pass full verification before merge to `main`

## Shared Constraints

- Reuse current APIs and adapters before introducing new subsystems
- Preserve `AGENTOS_MODE=dev` for local workflows
- Keep Open Core boundary clear: community-friendly foundations first, enterprise wrappers second
- Avoid overlapping file ownership where possible:
  - Track A owns UI, gateway-facing product flows, docs, examples
  - Track B owns auth, policy, tenant, approval, governance paths
  - Track C owns agent/tool packaging, templates, SDK/docs/integration adapters

### Task 1: Prepare isolated workspaces

**Files:**
- Modify: `.gitignore` if `.worktrees/` is not ignored
- Create: `.worktrees/<branch>/...` via `git worktree add`

**Step 1: Verify worktree directory choice**

Run: `git check-ignore -q .worktrees`
Expected: exit code `0` if `.worktrees/` is already ignored

**Step 2: Fix ignore rule if needed**

Add `.worktrees/` to `.gitignore` only if the previous check fails.

**Step 3: Create track worktrees**

Run:

```bash
git worktree add ".worktrees/feat-developer-platform-phase-1" -b feat/developer-platform-phase-1
git worktree add ".worktrees/feat-enterprise-governance-phase-1" -b feat/enterprise-governance-phase-1
git worktree add ".worktrees/feat-ecosystem-integrations-phase-1" -b feat/ecosystem-integrations-phase-1
git worktree add ".worktrees/integration-agent-team-phase-1" -b integration/agent-team-phase-1
```

**Step 4: Establish clean baselines**

Run in each worktree:

```bash
go test ./...
cargo test --manifest-path runtime/Cargo.toml
```

Expected: known clean baseline or a documented list of pre-existing failures.

### Task 2: Track A - Developer platform

**Files:**
- Modify: `cmd/apiserver/main.go`
- Modify: `internal/access/http/server.go`
- Modify: `internal/gateway/http.go`
- Create: `internal/console/...` or `web/...` depending on chosen UI location
- Modify: `docs/reference/api-surfaces.md`
- Modify: `docs/reference/api-surfaces-cn.md`
- Modify: `examples/...`

**Step 1: Write failing tests for product-facing endpoints**

Add tests for:
- task list / filter experience
- task detail projection
- worker status view
- agent catalog listing

**Step 2: Run focused tests**

Run: `go test ./internal/access/http ./internal/gateway -run Test`
Expected: FAIL for new behavior before implementation.

**Step 3: Implement minimal APIs and projections**

Add product-facing read models first; avoid changing runtime execution flow.

**Step 4: Add first usable console surface**

Implement a lightweight operator/developer console for:
- tasks
- replay/audit
- streaming logs
- agents
- workers

**Step 5: Verify track A**

Run:

```bash
go test ./internal/access/http ./internal/gateway ./internal/worker
```

**Step 6: Commit**

```bash
git add .
git commit -m "feat(product): add developer platform surfaces"
```

### Task 3: Track B - Enterprise governance

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `internal/access/...`
- Modify: `internal/policy/...`
- Modify: `internal/orchestration/...`
- Modify: `docs/reference/configuration.md`
- Modify: `docs/reference/configuration-cn.md`

**Step 1: Write failing tests for governance behavior**

Add tests for:
- multi-tenant isolation
- role-based access checks
- approval-required actions
- quota / rate enforcement

**Step 2: Run focused tests**

Run: `go test ./internal/access ./internal/policy ./internal/orchestration -run Test`
Expected: FAIL for missing governance behavior.

**Step 3: Implement minimal governance core**

Prioritize:
- tenant-aware access model
- role checks on read/write paths
- approval gate for risky tool/action categories
- usage accounting hooks

**Step 4: Expose admin configuration surfaces**

Support env/config-based setup first; postpone external IAM providers until the core policy model is stable.

**Step 5: Verify track B**

Run:

```bash
go test ./internal/access ./internal/policy ./internal/orchestration ./pkg/config
```

**Step 6: Commit**

```bash
git add .
git commit -m "feat(governance): add tenant and approval controls"
```

### Task 4: Track C - Ecosystem and integrations

**Files:**
- Modify: `internal/agent/...`
- Modify: `internal/tool/...`
- Create: `examples/templates/...`
- Create: `sdk/...` or `docs/guides/...` depending on implementation
- Modify: `README.md`
- Modify: `README_CN.md`

**Step 1: Write failing tests for packaging and integration flows**

Add tests for:
- reusable agent templates
- tool package registration
- import/export of agent definitions
- external integration adapter contracts

**Step 2: Run focused tests**

Run: `go test ./internal/agent ./internal/tool -run Test`
Expected: FAIL for new ecosystem features.

**Step 3: Implement minimum viable ecosystem layer**

Prioritize:
- starter templates for coding / review / ops agents
- importable tool bundles
- documented integration contract for Git and CI systems

**Step 4: Publish docs and examples**

Add copy-paste-ready examples before adding a large plugin framework.

**Step 5: Verify track C**

Run:

```bash
go test ./internal/agent ./internal/tool ./examples/...
```

**Step 6: Commit**

```bash
git add .
git commit -m "feat(ecosystem): add templates and integration contracts"
```

### Task 5: Integrate track branches

**Files:**
- Merge output from all three track branches into `integration/agent-team-phase-1`

**Step 1: Merge Track A**

Run: `git merge --no-ff feat/developer-platform-phase-1`

**Step 2: Merge Track B**

Run: `git merge --no-ff feat/enterprise-governance-phase-1`

**Step 3: Merge Track C**

Run: `git merge --no-ff feat/ecosystem-integrations-phase-1`

**Step 4: Resolve conflicts with ownership rules**

If two tracks touch the same boundary, keep the narrowest API change and update docs/tests immediately.

**Step 5: Run integration verification**

Run:

```bash
go test ./...
cargo test --manifest-path runtime/Cargo.toml
```

**Step 6: Run end-to-end verification**

Run: `./scripts/acceptance.sh`
Expected: controller + apiserver + worker full flow passes.

**Step 7: Commit integration fixes if needed**

```bash
git add .
git commit -m "fix(integration): reconcile parallel track changes"
```

### Task 6: Merge to main

**Files:**
- Merge verified `integration/agent-team-phase-1` into `main`

**Step 1: Verify integration branch is green**

Collect command outputs from:
- `go test ./...`
- `cargo test --manifest-path runtime/Cargo.toml`
- `./scripts/acceptance.sh`

**Step 2: Merge to main**

Run:

```bash
git checkout main
git merge --no-ff integration/agent-team-phase-1
```

**Step 3: Final post-merge verification**

Run the same verification commands again on `main`.

**Step 4: Tag phase completion in docs/changelog**

Update release notes only after successful verification.

## Execution Order

1. Create worktrees and establish baselines
2. Dispatch one subagent per track in parallel
3. Review each branch separately
4. Merge into integration branch
5. Run full verification
6. Merge integration branch into `main`

## Done Criteria

- Developer platform track produces a usable product surface for R&D teams
- Governance track enforces tenant, role, and approval boundaries
- Ecosystem track ships reusable templates and documented integration contracts
- Full Go and Rust tests pass
- `scripts/acceptance.sh` passes
- `main` contains the verified integrated result
