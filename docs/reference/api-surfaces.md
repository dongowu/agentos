# API Surfaces Reference

This page summarizes the current HTTP and SSE surfaces exposed by AgentOS.

It is intended as a practical reference for the **current open-source platform shape**, not as a future-facing design sketch.

## Authentication Model

- `GET /health` is public
- `/v1/*` routes require a bearer token when `AuthProvider` is configured
- gateway routes (`/agent/run`, `/agent/status`, `/agent/list`, `/tool/run`) also require a bearer token when auth is configured
- when auth is disabled, the server allows unauthenticated access
- tenant-aware reads are enforced on task, audit, replay, and stream paths when tenant context is available

### Bearer Header

```http
Authorization: Bearer <token>
```

### Common Auth Errors

```json
{"error":"missing bearer token"}
```

```json
{"error":"unauthorized"}
```

```json
{"error":"forbidden"}
```

## Core HTTP API

### Health

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/health` | liveness / basic health check |

Example response:

```json
{"status":"ok"}
```

### Task Submission And Read Paths

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/v1/tasks` | create a task |
| `GET` | `/v1/tasks/{task_id}` | fetch task state |
| `GET` | `/v1/tasks/{task_id}/audit` | list persisted audit records for a task |
| `GET` | `/v1/tasks/{task_id}/actions/{action_id}/audit` | fetch one action audit record |
| `GET` | `/v1/tasks/{task_id}/replay` | fetch task-centric replay projection |
| `GET` | `/v1/tasks/{task_id}/stream` | task-level SSE telemetry |
| `GET` | `/v1/tasks/{task_id}/actions/{action_id}/stream` | action-level SSE telemetry |
| `GET` | `/v1/audit` | query platform-level audit feed |

### `POST /v1/tasks`

Request body:

```json
{
  "prompt": "echo hello",
  "tenant_id": "tenant-a",
  "agent_name": "ops-agent"
}
```

Notes:

- `prompt` is required
- `tenant_id` is optional in the body, but when auth provides tenant context it is auto-filled or tenant-checked
- `agent_name` is optional and lets the control plane associate the task with an agent profile

Success response:

```json
{
  "task_id": "task-123",
  "state": "queued"
}
```

Common errors:

```json
{"error":"invalid json"}
```

```json
{"error":"prompt required"}
```

```json
{"error":"tenant mismatch"}
```

### `GET /v1/tasks/{task_id}`

Success response:

```json
{
  "task_id": "task-123",
  "state": "running"
}
```

### `GET /v1/tasks/{task_id}/audit`

Success response shape:

```json
{
  "task_id": "task-123",
  "records": [
    {
      "task_id": "task-123",
      "action_id": "act-1",
      "command": "echo hello",
      "runtime_env": "native",
      "worker_id": "worker-1",
      "exit_code": 0,
      "stdout": "hello",
      "stderr": "",
      "occurred_at": "2026-03-09T00:00:00Z"
    }
  ]
}
```

### `GET /v1/tasks/{task_id}/actions/{action_id}/audit`

Success response is a single `AuditRecord` object.

### `GET /v1/tasks/{task_id}/replay`

Success response shape:

```json
{
  "task_id": "task-123",
  "state": "succeeded",
  "tenant_id": "tenant-a",
  "agent_name": "ops-agent",
  "prompt": "fix deployment",
  "summary": {
    "action_count": 2,
    "completed_count": 2,
    "failed_count": 1
  },
  "actions": [
    {
      "action_id": "act-1",
      "status": "completed",
      "command": "echo hello",
      "stdout": "hello"
    }
  ]
}
```

### `GET /v1/audit`

Supported query parameters:

| Query | Purpose |
|-------|---------|
| `task_id` | filter by task id |
| `action_id` | filter by action id |
| `tenant_id` | filter by tenant id |
| `agent_name` | filter by agent name |
| `worker_id` | filter by worker id |
| `failed` | `true/1` or `false/0` |
| `limit` | max record count |

Notes:

- when auth provides tenant context, the authenticated tenant overrides any incoming `tenant_id`
- invalid `failed` or `limit` values return `400`

Success response shape:

```json
{
  "records": [
    {
      "task_id": "task-2",
      "action_id": "act-1",
      "tenant_id": "tenant-a",
      "agent_name": "ops",
      "exit_code": 1,
      "error": "failed",
      "occurred_at": "2026-03-09T00:00:00Z"
    }
  ]
}
```

## Gateway API

These routes sit alongside the core `/v1/*` control-plane API and provide a more agent- or tool-oriented façade.

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/agent/run` | submit a task through an agent-facing entrypoint |
| `GET` | `/agent/status?task_id=...` | fetch task state through the gateway |
| `GET` | `/agent/list` | list available agent names |
| `POST` | `/tool/run` | invoke a built-in tool directly |

### `POST /agent/run`

Request body:

```json
{
  "agent": "demo",
  "task": "echo hello"
}
```

Success response:

```json
{
  "task_id": "task-123",
  "state": "queued",
  "agent": "demo"
}
```

Notes:

- `task` is required
- `agent` is optional
- when an agent runtime can build an agent-aware prompt, the gateway forwards that richer prompt into task creation
- unknown agents return `404`

### `GET /agent/status?task_id=...`

Success response:

```json
{
  "task_id": "task-123",
  "state": "running"
}
```

### `GET /agent/list`

Success response:

```json
{
  "agents": ["demo", "coder"]
}
```

### `POST /tool/run`

Request body:

```json
{
  "tool": "file.read",
  "input": {
    "path": "README.md"
  }
}
```

Success response shape:

```json
{
  "result": {}
}
```

Notes:

- `tool` is required
- `input` is optional and defaults to an empty object
- tool execution failures currently return `400`

## SSE Streams

### Task Stream

`GET /v1/tasks/{task_id}/stream`

Headers:

```http
Content-Type: text/event-stream; charset=utf-8
Cache-Control: no-cache
Connection: keep-alive
```

Current event sequence rules:

- the stream begins with a `task.snapshot` event containing current task state
- if persisted audit records already exist, the server replays action output and completion events first
- if the task is already terminal (`succeeded` or `failed`), the stream closes after snapshot + replay
- otherwise the server subscribes to live task events

Current task-stream event names:

- `task.snapshot`
- `task.created`
- `task.planned`
- `task.action.dispatched`
- `task.action.output`
- `task.action.completed`

### Action Stream

`GET /v1/tasks/{task_id}/actions/{action_id}/stream`

Current action-stream behavior:

- if an audit record already exists, the server replays `task.action.output` and `task.action.completed` and then closes
- otherwise it subscribes to live `task.action.output` and `task.action.completed`
- the action stream closes after `task.action.completed`

### Event Payload Types

Representative payloads:

```json
{
  "task_id": "task-123",
  "action_id": "act-1",
  "kind": "stdout",
  "text": "hello",
  "occurred": "2026-03-09T00:00:00Z"
}
```

```json
{
  "task_id": "task-123",
  "action_id": "act-1",
  "exit_code": 0,
  "stdout": "hello",
  "stderr": "",
  "worker_id": "worker-1",
  "occurred": "2026-03-09T00:00:01Z"
}
```

## Error Envelope

Most error responses use a simple JSON envelope:

```json
{"error":"message"}
```

Typical status codes:

- `200` — success
- `400` — bad input or invalid query value
- `401` — missing / invalid bearer token
- `403` — tenant mismatch or forbidden cross-tenant read
- `404` — missing task, missing agent, or missing replay source
- `405` — method not allowed
- `500` — API, audit store, event bus, or gateway dependency not configured

## Read Next

- [Core Capabilities Reference](core-capabilities.md)
- [Getting Started Guide](../guides/getting-started.md)
- [Architecture Overview](../architecture/overview.md)
