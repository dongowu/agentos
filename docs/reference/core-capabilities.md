# Core Capabilities Reference

This page captures the detailed capability reference that is no longer carried inline on the homepage README.

## Agent DSL

Agents are config, not code:

```yaml
name: defi-trading-agent
description: "Monitors markets and executes trades."
model: gpt-4o

memory:
  type: redis
  ttl: 86400

tools:
  - http.get
  - http.post
  - shell

policy:
  allow: ["http.*"]
  deny: ["shell"]

workflow:
  - plan
  - execute
  - reflect
```

## Built-in Tools

| Tool | Description |
|------|-------------|
| `shell` | Execute shell commands with sandbox-aware runtime handling |
| `file.read` | Read file contents |
| `file.write` | Write files and create missing directories |
| `git.clone` | Clone Git repositories |
| `git.status` | Inspect Git working tree status |
| `http.get` | HTTP GET requests |
| `http.post` | HTTP POST requests |

## Pluggable Adapters

| Interface | Adapters | Default |
|-----------|----------|---------|
| `EventBus` | memory, nats | prod: `nats`; dev: `memory` |
| `TaskRepository` | memory, postgres | prod: `postgres`; dev: `memory` |
| `AuditLogStore` | memory, postgres | prod: `postgres`; dev: `memory` |
| `Planner` | prompt, registry-backed LLM providers (`openai` built in) | prompt baseline; bounded retry and repair before fallback |
| `Memory.Provider` | inmemory, redis | `inmemory` |
| `RuntimeAdapter` (Rust) | native, docker | `native` |
| `Scheduler` | local, nats | prod: `nats`; dev: `local` |

## Security Model

### Go control plane

- `PolicyEngine` enforces allow / deny rules with deny-first precedence
- `PolicyEngine` can block configured tool patterns with an `approval required` governance-gate reason
- autonomy levels cover `supervised`, `semi`, and `autonomous` modes
- `CredentialVault` uses opaque agent tokens so secrets stay out of general task payloads
- per-agent rate limiting constrains action volume
- dangerous command checks block high-risk patterns before execution

### Rust worker runtime

- `SecurityPolicy` validates commands and paths before execution
- environment isolation clears the process env and only re-adds safe variables
- secret redaction masks API keys, bearer tokens, and similar patterns in output
- output truncation limits result size
- per-action timeout enforcement protects worker health
- Docker runtime can enforce `--read-only`, `--network none`, and resource limits

## Distributed Architecture

At a high level:

- `controller` owns the shared registry and worker coordination path
- `apiserver` receives HTTP tasks, audit requests, replay requests, and SSE subscriptions
- the orchestration core performs planning, policy checks, dispatch, and result processing
- worker selection happens through the shared registry / pool path
- NATS-backed scheduling is available for distributed mode; local dispatch remains available for development

For an end-to-end proof, run [`./scripts/acceptance.sh`](../../scripts/acceptance.sh) and read [Multiprocess Acceptance](../architecture/multiprocess-acceptance.md).

## Repository Layout

- `api/` — protobuf contracts and generated gRPC bindings
- `cmd/` — entrypoints such as `apiserver`, `controller`, `claw-cli`, and `osctl`
- `internal/access/` — HTTP handlers, CLI wiring, and auth
- `internal/adapters/` — LLM, memory, messaging, persistence, and runtime clients
- `internal/agent/` — agent DSL, runtime metadata, and manager wiring
- `internal/bootstrap/` — dependency wiring from configuration
- `internal/orchestration/` — task engine, planners, and state-machine logic
- `internal/policy/` — policy engine and credential-vault logic
- `internal/scheduler/` — local and NATS-backed scheduling paths
- `internal/tool/` — built-in tool implementations and tool bridge surfaces
- `internal/worker/` — registry, pool, and health-monitor logic
- `pkg/` — shared config, events, and task DSL types
- `runtime/` — Rust worker, sandbox adapters, and telemetry crates
- `deploy/` — local infrastructure manifests such as NATS and Postgres compose files
- `examples/` — sample agents and tasks

## Open Core Boundary

AgentOS publishes the platform core under `Apache-2.0` and keeps commercial packaging outside the repository boundary.

- **Community** — self-hosted control plane, worker runtime, scheduling, audit APIs, replay, telemetry, and agent-loop substrate
- **Enterprise (future)** — org governance, SSO / SCIM / RBAC, long-retention audit center, support workflows
- **Cloud (future)** — hosted control plane, operator console, upgrades, billing, and SLA surfaces

See [Licensing Decision](../strategy/licensing-decision.md) and [Platform vs Capability Boundary](../architecture/platform-vs-capability-boundary.md) for the current boundary definition.
