# Product Direction: Governed Run

- Status: adopted as the working product direction for `v0.2`
- Date: 2026-11-16
- Audience: product, platform, runtime, and developer-experience maintainers

## Decision

AgentOS is a **self-hosted governed execution plane for internal coding and operations agents**.

It is not a general-purpose agent framework, a chat product, a workflow marketplace, or an enterprise console.

The initial ideal customer profile is a platform, infrastructure, or engineering-productivity team that wants to run internal coding or operations automation on infrastructure it controls.

The product promise is:

> Every agent run produces reviewable execution evidence: who ran it, which tool or action ran, how policy treated it, where it executed, what it returned, and how the run can be replayed.

## Why This Wedge

The repository already has the assets that are expensive to retrofit:

- a Go control plane and Rust worker runtime
- local and NATS-backed scheduling
- worker registration, health, and capacity reporting
- policy evaluation and credential isolation
- audit records, replay projections, and SSE telemetry
- a real multiprocess acceptance path

The missing product coherence is not another planner or another tool. It is a single, enforced governance boundary around the default agent execution path.

Frameworks help developers describe agents. AgentOS should make those agents safe and inspectable to operate. That is a narrower, more defensible product than competing on prompt authoring or tool breadth.

## The Golden Path

The first evaluation should answer one question in under ten minutes:

> Can I run an agent task and inspect exactly what happened?

The supported path is:

```bash
# Terminal 1: start the execution worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: run the task and print its replay
AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051 \
  go run ./cmd/osctl run "echo hello"

# Optional machine-readable evidence after starting the multiprocess stack
go run ./cmd/osctl --server http://127.0.0.1:18080 --token dev-token replay <task-id> --json
```

`osctl run` submits a task, waits for a terminal state, and renders the replay. A successful first run should show the task state, action count, command, exit code, worker/runtime metadata when available, and captured output. In standalone embedded mode the replay is available in that same invocation; later queries through `osctl replay` require a connected API with retained task state.

## Product Contract

The following invariants define whether a run is governed:

1. Every externally visible tool call is evaluated by policy before it executes.
2. A denied call does not execute and leaves an audit record explaining the denial.
3. Every executed tool call records its action, outcome, and relevant runtime context.
4. Completed runs can be inspected through a replay projection.
5. The default CLI and API path exposes that evidence without requiring direct database access.

Local development shortcuts may exist, but they must be explicit. They must not be described as governed execution.

## Release Scope

### Now: make governance visible

- make `osctl run` and `osctl replay` the primary first-run experience
- apply policy evaluation and audit recording to the LLM tool-calling path
- keep `agentos-worker` in the planned-action execution path
- keep examples limited to tools that actually exist
- extend acceptance coverage to include allowed execution, denied execution, and replay

### Next: make the agent profile a real contract

- reject unregistered tools instead of silently dropping them
- make agent `tools`, `policy`, `model`, and `memory` declarations affect runtime behavior
- provide one reference coding/ops workflow using only the supported file, Git, shell, and HTTP tools
- route process-capable tool calls through the worker boundary where required for sandbox and audit guarantees

### Later: prove repeat use

- ship a documented `v0.2` governed-run contract
- run a small self-hosted pilot with internal engineering teams
- publish only measured outcomes, not broad platform claims

## Success Metrics

These are release gates, not current measurements:

| Area | Target |
|------|--------|
| Activation | a new developer completes a governed run and opens its replay in under 10 minutes |
| Governance | 100% of tool calls have a policy decision and audit record |
| Denial safety | 100% of denied calls have no observed side effect |
| Replayability | at least 95% of completed pilot runs can be replayed from persisted evidence |
| Workload quality | at least 80% success on the documented reference coding/ops workload |
| Adoption | at least two pilot teams run governed tasks weekly |

If these gates are missed, improve the governance path before adding tools, consoles, or vertical packs.

## Non-Goals

- a general chat product or personal assistant
- a prompt IDE, low-code workflow editor, or agent marketplace
- browser automation, Web3 trading, or vertical industry packs in the core
- a production dashboard or enterprise console in this release cycle
- enterprise SSO, SCIM, RBAC, organization management, or long-retention audit services in the open-source core
- multi-agent orchestration as a headline capability

## Product Boundaries

### Open-source core

Execution orchestration, runtime, policy, audit, replay, worker registry, scheduling, telemetry, CLI, and the reference developer experience.

### Commercial future

Organization governance, identity integration, managed controls, long-retention audit operations, hosted infrastructure, and support.

Enterprise capabilities should extend the platform through interfaces. They should not fork the execution or governance model.

## Deferred Decisions

These decisions are intentionally deferred because they do not block the governed-run wedge:

- the first external identity provider
- a hosted cloud packaging model
- whether non-coding vertical packs become a separate product line
- the long-term relationship between `claw` and `osctl`

Until those decisions are made, `osctl` is the primary operator interface and ClawOS material remains conceptual or compatibility-oriented.

## Acceptance Evidence For This Direction

The direction is considered implemented for this cycle when all of the following are true:

- `osctl run` reaches a terminal state and prints replay evidence
- `osctl replay --json` returns the complete replay contract
- a denied tool call is blocked before execution and recorded in audit
- an allowed LLM tool call is policy-checked and recorded in audit
- the standard Go test suite and multiprocess acceptance path remain green

