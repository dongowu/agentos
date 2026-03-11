# Runtime And Sandbox Reference

This page documents the current runtime plane of AgentOS: how actions are executed, how sandboxing works today, and what guarantees the worker runtime actually provides.

It describes the **current implemented surface**, not future-facing runtime ideas.

## Runtime Plane Overview

The runtime plane is the Rust execution side of AgentOS.

At a high level it contains:

- `runtime/crates/worker` — the gRPC worker service
- `runtime/crates/sandbox` — runtime adapters and security enforcement
- `runtime/crates/telemetry` — stream payload models
- `internal/adapters/runtimeclient` — the Go control-plane gRPC client
- `internal/worker` — registry, pool, and worker selection on the Go side

## Current Execution Model

A control-plane action reaches the runtime plane like this:

1. the Go control plane chooses a worker
2. it sends a gRPC request to `RuntimeService`
3. the Rust worker converts the action payload into an `ExecutionSpec`
4. `ActionExecutor` applies `SecurityPolicy`
5. the selected runtime adapter executes the command
6. stdout / stderr are truncated if needed and secrets are redacted
7. the worker returns either a one-shot result or streamed chunks

## gRPC Runtime Surface

The worker currently exposes two RPCs:

| RPC | Purpose |
|-----|---------|
| `ExecuteAction` | execute and return final stdout / stderr / exit code |
| `StreamOutput` | stream stdout / stderr chunks and finish with an exit chunk |

Proto contract summary:

```proto
service RuntimeService {
  rpc ExecuteAction(ExecuteActionRequest) returns (ExecuteActionResponse);
  rpc StreamOutput(StreamOutputRequest) returns (stream StreamChunk);
}
```

Important payload fields:

- `task_id`
- `action_id`
- `payload` / runtime payload bytes
- streamed `StreamChunk.kind` values such as `stdout`, `stderr`, and `resource`

## Execution Contract

### `ExecutionSpec`

Each runtime executes an `ExecutionSpec` with these effective fields:

| Field | Meaning |
|-------|---------|
| `command` | shell command string |
| `working_dir` | optional working directory |
| `env` | environment variables injected after isolation |
| `timeout` | wall-clock timeout |
| `max_output_bytes` | output truncation limit |

### `ExecutionResult`

Every completed action returns:

| Field | Meaning |
|-------|---------|
| `exit_code` | process exit code |
| `stdout` | captured stdout bytes |
| `stderr` | captured stderr bytes |
| `duration` | execution duration |
| `truncated` | whether output was truncated |

## Runtime Adapters

### Native runtime

Current characteristics:

- runtime name: `native`
- executes commands directly on the host OS
- detects a usable shell at startup
- on non-Windows systems it prefers `sh` and then `bash`
- on Windows it can fall back through `bash`, `sh`, `cmd`, and `COMSPEC`
- supports working directory changes via `current_dir(...)`
- clears the process environment before re-injecting a small safe allowlist plus caller env

Current safe env passthrough list:

- `PATH`
- `HOME`
- `TERM`
- `LANG`
- `USER`
- `SHELL`
- `TMPDIR`

What native runtime means operationally:

- it has shell access
- it has host filesystem access
- it is the fastest local path
- it is the least isolated path compared with container execution

### Docker runtime

Current characteristics:

- runtime name: `docker`
- builds a `docker run` command line
- uses `--rm` and `--init`
- supports explicit network mode
- supports memory and CPU limits
- can enable `--read-only`
- can optionally mount the host workspace into `/workspace`
- passes the safe env allowlist and caller env through `--env`

Current command-shape features:

- `--network <mode>` when configured
- `--memory <n>m` when configured
- `--cpus <limit>` when configured
- `--read-only` when enabled
- `--volume <host>:/workspace:rw` + `--workdir /workspace` when workspace mounting is enabled
- image + `sh -c <command>` as the final execution form

Current mount safety checks:

- mounting requires `working_dir` in the execution spec
- the working directory must resolve to an absolute path
- mounting `/` is explicitly rejected
- if `allowed_workspace_roots` is configured, mounts must stay inside those roots

Default Docker posture from worker env defaults:

- image: `ubuntu:22.04`
- network: `none`
- memory: `512 MB`
- CPU: `1.0`
- workspace mount: disabled
- read-only rootfs: disabled

## Security Policy

The Rust worker enforces command validation through `SecurityPolicy` before execution.

### Autonomy levels

| Level | Behavior |
|-------|----------|
| `supervised` | only whitelisted commands are allowed; unknown commands are denied |
| `semi` / `semi_autonomous` | also whitelist-driven, but conceptually positioned between supervised and autonomous |
| `autonomous` | allows commands unless they match explicit deny rules |

### Default whitelist examples

The default whitelist includes base commands such as:

- `ls`
- `cat`
- `echo`
- `pwd`
- `head`
- `tail`
- `grep`
- `find`
- `wc`

### Default blacklist examples

The default blacklist includes patterns such as:

- `rm -rf /`
- `rm -rf /*`
- `mkfs.*`
- `dd if=/dev/*`
- `chmod 777 *`
- `:(){ :|:& };:`

### Forbidden path checks

By default, the policy blocks references to sensitive paths such as:

- `/etc/shadow`
- `/etc/passwd`

### Output and rate limits

Default policy limits include:

- `max_actions_per_hour = 120`
- `max_output_bytes = 1_048_576`

## Secret Redaction

Before output leaves the worker, the security layer redacts likely secrets.

Current redaction targets include patterns resembling:

- API keys such as `sk-...`
- bearer tokens
- AWS access keys
- GitHub tokens like `ghp_...`
- generic `key=...`, `token=...`, `secret=...`, `password=...` forms

Redacted output is replaced with:

```text
[REDACTED]
```

## Output Truncation

Both one-shot execution and streaming paths enforce output limits.

Current behavior:

- output is truncated to `max_output_bytes`
- truncation tries to preserve UTF-8 boundaries
- a truncation marker is appended

Current truncation marker:

```text
... [output truncated]
```

In streaming mode, once the limit is reached, the worker emits a truncation chunk and stops forwarding additional data.

## Streaming Behavior

When the control plane uses the streaming runtime path:

- native and docker runtimes can both stream stdout / stderr
- chunks are emitted with `kind = stdout` or `kind = stderr`
- secret redaction is applied before chunks are emitted
- the stream shares the same output budget accounting
- timeouts are translated into gRPC deadline-style errors

Representative chunk shape:

```json
{
  "task_id": "task-123",
  "action_id": "act-1",
  "kind": "stdout",
  "data": "aGVsbG8="
}
```

## Worker Registration And Heartbeats

When `AGENTOS_CONTROL_PLANE_ADDR` is set, the worker runtime also participates in control-plane registration.

Current behavior:

- the worker starts its gRPC service
- it registers itself with the controller `WorkerRegistry`
- it advertises a worker id, listen address, capabilities, and max task count
- it starts a heartbeat loop at the configured interval
- duplicate registration refreshes the controller-side worker snapshot instead of failing hard
- if heartbeats are rejected because the controller forgot the worker, the runtime treats that as a registration failure and re-registers
- if no control-plane address is set, it skips registration and still serves execution locally

Operationally, this means a worker restart with the same worker id is expected to converge back to a healthy online record without manual controller cleanup.

## Current Stable Runtime Surface

Today the runtime plane should be treated as stable around these capabilities:

- `native` runtime
- `docker` runtime
- `SecurityPolicy` command validation
- secret redaction
- output truncation
- one-shot execute RPC
- streaming output RPC
- worker registration + heartbeat against the shared controller registry

## What Is Not Currently A Stable Runtime Surface

These ideas exist in historical or concept docs, but are **not** part of the current stable runtime implementation:

- gVisor runtime
- Firecracker runtime
- WASM runtime
- browser-specialized runtime in the Rust worker
- full console-grade terminal session streaming

## Legacy And Deprecated Pieces

There is still a deprecated compatibility layer in the sandbox crate:

- `SandboxSpec`
- `SandboxHandle`
- `IsolationProvider`
- legacy `WorkerService`

Current code should prefer:

- `ExecutionSpec`
- `ExecutionResult`
- `RuntimeAdapter`
- `ActionExecutor`

## Read Next

- [Configuration Reference](configuration.md)
- [API Surfaces Reference](api-surfaces.md)
- [Core Capabilities Reference](core-capabilities.md)
- [Architecture Overview](../architecture/overview.md)
