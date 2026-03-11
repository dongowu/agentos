# API 能力面参考

这份文档汇总了 AgentOS 当前暴露的 HTTP 与 SSE 接口面。

它描述的是**当前开源平台的实际能力面**，不是未来态设计草图。

## 鉴权模型

- `GET /health` 和 `GET /ready` 为公开接口
- 当服务配置了 `AuthProvider` 时，`/v1/*` 路径要求 Bearer Token
- gateway 路径（`/agent/run`、`/agent/status`、`/agent/list`、`/tool/run`）在启用鉴权时也要求 Bearer Token
- 如果未启用鉴权，服务允许匿名访问
- 当请求上下文中存在 tenant 信息时，task / audit / replay / stream 读路径会执行租户级校验

### Bearer Header

```http
Authorization: Bearer <token>
```

### 常见鉴权错误

```json
{"error":"missing bearer token"}
```

```json
{"error":"unauthorized"}
```

```json
{"error":"forbidden"}
```

## 核心 HTTP API

### 健康检查与就绪检查

| 方法 | 路径 | 用途 |
|------|------|------|
| `GET` | `/health` | 存活检查 + scheduler / worker 容量摘要 |
| `GET` | `/ready` | 基于同一份容量摘要的可调度就绪检查 |

示例响应：

```json
{
  "status": "ok",
  "scheduler_mode": "nats",
  "recovery_enabled": true,
  "capacity_warnings": ["no available workers for capability docker"],
  "workers": {
    "total": 3,
    "online": 1,
    "busy": 1,
    "offline": 1,
    "available_workers": 1
  }
}
```

运行说明：

- `/health` 在 HTTP 服务存活时始终返回 `200`，即使响应体里显示 worker 容量已降级
- `/ready` 只有在 worker registry 可读取且至少存在一个可调度 worker 槽位时才返回 `200`；否则返回 `503`
- `status = degraded` 表示控制面当前没有可调度 worker 容量，或者无法读取 worker registry
- `capacity_warnings` 用于提示 capability 级别的饥饿/耗尽，例如 `docker` 仍有注册 worker，但已经没有可调度槽位

### 任务提交与读取路径

| 方法 | 路径 | 用途 |
|------|------|------|
| `POST` | `/v1/tasks` | 创建任务 |
| `GET` | `/v1/tasks/{task_id}` | 获取任务状态 |
| `GET` | `/v1/tasks/{task_id}/audit` | 获取任务级持久化审计记录 |
| `GET` | `/v1/tasks/{task_id}/actions/{action_id}/audit` | 获取单个 action 审计记录 |
| `GET` | `/v1/tasks/{task_id}/replay` | 获取任务回放投影 |
| `GET` | `/v1/tasks/{task_id}/stream` | 任务级 SSE 遥测流 |
| `GET` | `/v1/tasks/{task_id}/actions/{action_id}/stream` | 动作级 SSE 遥测流 |
| `GET` | `/v1/audit` | 查询平台级审计流 |
| `GET` | `/v1/workers` | 查看当前 worker registry 快照 |

### `POST /v1/tasks`

请求体：

```json
{
  "prompt": "echo hello",
  "tenant_id": "tenant-a",
  "agent_name": "ops-agent"
}
```

说明：

- `prompt` 必填
- `tenant_id` 在 body 中可选；如果鉴权上下文里有 tenant，会自动补齐或校验一致性
- `agent_name` 可选，用于把任务与某个 agent profile 关联起来

成功响应：

```json
{
  "task_id": "task-123",
  "state": "queued"
}
```

常见错误：

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

成功响应：

```json
{
  "task_id": "task-123",
  "state": "running"
}
```

### `GET /v1/tasks/{task_id}/audit`

成功响应结构：

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

成功响应就是单个 `AuditRecord` 对象。

### `GET /v1/tasks/{task_id}/replay`

成功响应结构：

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

支持的查询参数：

| 参数 | 用途 |
|------|------|
| `task_id` | 按 task id 过滤 |
| `action_id` | 按 action id 过滤 |
| `tenant_id` | 按 tenant id 过滤 |
| `agent_name` | 按 agent name 过滤 |
| `worker_id` | 按 worker id 过滤 |
| `failed` | `true/1` 或 `false/0` |
| `limit` | 返回的最大记录数 |

说明：

- 如果鉴权上下文里带 tenant，服务端会用已认证 tenant 覆盖传入的 `tenant_id`
- `failed` 或 `limit` 非法时会返回 `400`

成功响应结构：

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

### `GET /v1/workers`

支持的查询参数：

| 参数 | 用途 |
|------|------|
| `available_only` | 当为 `true/1` 时，仅返回当前可调度的 worker |
| `status` | 仅返回 `status` 与给定值精确匹配的 worker，例如 `online` |
| `capability` | 仅返回声明了指定能力的 worker，例如 `native` |

成功响应结构：

```json
{
  "summary": {
    "total": 2,
    "online": 1,
    "busy": 1,
    "offline": 0,
    "available_workers": 1,
    "capabilities": [
      {
        "name": "docker",
        "total": 1,
        "online": 0,
        "busy": 1,
        "offline": 0,
        "available_workers": 0
      },
      {
        "name": "native",
        "total": 1,
        "online": 1,
        "busy": 0,
        "offline": 0,
        "available_workers": 1
      }
    ]
  },
  "workers": [
    {
      "id": "worker-1",
      "addr": "127.0.0.1:5001",
      "capabilities": ["native"],
      "status": "online",
      "last_heartbeat": "2026-03-11T00:00:00Z",
      "active_tasks": 0,
      "max_tasks": 2
    }
  ]
}
```

说明：

- 该接口与其他 `/v1/*` 路径一样，遵循相同的 Bearer Token 鉴权规则
- `summary` 的统计语义与 `/health`、`/ready` 保持一致，但作用于当前返回的 worker 集合
- `summary.capabilities` 会按 capability 名称排序，给出每个已声明 capability 的同口径统计
- 如果一个 worker 同时声明多个 capability，它会分别计入每个匹配 capability 的桶中
- 查询参数按 `AND` 组合，因此 `available_only=true&status=online&capability=native` 会返回三者交集
- `available_only` 非法时返回 `400`

## CLI 机器可读 schema

仓库也对本地运维和自动化场景提供了稳定的机器可读 CLI 诊断输出。

### `claw dev --output json`

默认结构：

```json
{
  "schema_version": "v1",
  "health": { "status": "ok" },
  "ready": { "status": "ok" },
  "workers": {
    "summary": {
      "total": 1,
      "online": 1,
      "busy": 0,
      "offline": 0,
      "available_workers": 1,
      "capabilities": [
        {
          "name": "native",
          "total": 1,
          "online": 1,
          "busy": 0,
          "offline": 0,
          "available_workers": 1
        }
      ]
    },
    "workers": [
      {
        "id": "worker-1",
        "addr": "127.0.0.1:5001",
        "capabilities": ["native"],
        "status": "online",
        "active_tasks": 0,
        "max_tasks": 2
      }
    ]
  },
  "agents": ["demo"]
}
```

说明：

- `schema_version` 当前固定为 `v1`
- `--section` 会把输出裁剪到所选的顶层字段，同时保留 `schema_version`
- `--section` 支持单个值、逗号分隔值（例如 `health,workers`），也支持重复传入 flag
- 当前支持的 section 为 `health`、`ready`、`workers`、`agents`
- `--require-ready` 会继续输出诊断内容，但只要 `/ready` 不是 `status=ok` 就返回非零退出码
- `--require-capability` 支持重复传入或逗号分隔；只要任一所需 capability 缺失或 `available_workers <= 0`，就返回非零退出码

### `osctl workers --output json`

默认结构：

```json
{
  "schema_version": "v1",
  "summary": {
    "total": 1,
    "online": 1,
    "busy": 0,
    "offline": 0,
    "available_workers": 1,
    "capabilities": [
      {
        "name": "native",
        "total": 1,
        "online": 1,
        "busy": 0,
        "offline": 0,
        "available_workers": 1
      }
    ]
  },
  "workers": [
    {
      "id": "worker-1",
      "addr": "127.0.0.1:5001",
      "capabilities": ["native"],
      "status": "online",
      "active_tasks": 0,
      "max_tasks": 2
    }
  ]
}
```

说明：

- `schema_version` 当前固定为 `v1`
- `--summary-only` 只保留 `schema_version` 与 `summary`
- `--workers-only` 只保留 `schema_version` 与 `workers`
- 面向人工的 table 输出还支持 `--no-capability-summary` 与 `--no-workers`，可裁剪终端显示而不改变 JSON 合约
- `--unschedulable-only` 只保留当前不可调度的 worker（例如 `status != online`、零容量或已满载）
- `--sort` 支持 `id`、`load`、`status`，`--limit` 会在过滤/排序后裁剪 worker 列表
- `--require-count` 会在输出 worker 数量少于给定值时返回非零退出码
- `--require-available-count` 会在输出 summary 里的可调度 worker 数量少于给定值时返回非零退出码
- `--require-load-threshold` 会在当前输出任一 worker 的归一化负载（`active_tasks / max_tasks`）超过给定阈值时返回非零退出码
- `--require-worker` 支持重复传入或逗号分隔；只要任一指定 worker id 不在当前输出结果中，就返回非零退出码
- `--require-capability-count` 支持以 `capability=count` 形式重复传入或逗号分隔；只要当前输出的 worker 子集中某个 capability 数量低于要求，就返回非零退出码
- `--require-capability-available-count` 支持以 `capability=count` 形式重复传入或逗号分隔；只要当前输出 capability 摘要里的可调度 worker 数量低于要求，就返回非零退出码
- `--require-capability-online-count`、`--require-capability-busy-count`、`--require-capability-offline-count` 支持以 `capability=count` 形式重复传入或逗号分隔；只要当前输出 capability 摘要里的 `online` / `busy` / `offline` 数量低于要求，就返回非零退出码
- `--require-status-count` 支持以 `status=count` 形式重复传入或逗号分隔；只要当前输出的 worker 子集中 `online` / `busy` / `offline` 数量低于要求，就返回非零退出码
- 当 CLI 侧应用过滤或 limit 时，输出中的 `summary` 会重算，并与当前实际输出的 worker 子集保持一致

## Gateway API

这些路径和核心 `/v1/*` 控制面 API 并存，提供一个更偏 agent / tool 的入口层。

| 方法 | 路径 | 用途 |
|------|------|------|
| `POST` | `/agent/run` | 通过 agent 入口提交任务 |
| `GET` | `/agent/status?task_id=...` | 通过 gateway 查询任务状态 |
| `GET` | `/agent/list` | 列出可用 agent 名称 |
| `POST` | `/tool/run` | 直接调用内置 tool |

### `POST /agent/run`

请求体：

```json
{
  "agent": "demo",
  "task": "echo hello"
}
```

成功响应：

```json
{
  "task_id": "task-123",
  "state": "queued",
  "agent": "demo"
}
```

说明：

- `task` 必填
- `agent` 可选
- 如果 agent runtime 能构建 agent-aware prompt，gateway 会把增强后的 prompt 转发到任务创建路径
- 未知 agent 返回 `404`

### `GET /agent/status?task_id=...`

成功响应：

```json
{
  "task_id": "task-123",
  "state": "running"
}
```

### `GET /agent/list`

成功响应：

```json
{
  "agents": ["demo", "coder"]
}
```

### `POST /tool/run`

请求体：

```json
{
  "tool": "file.read",
  "input": {
    "path": "README.md"
  }
}
```

成功响应结构：

```json
{
  "result": {}
}
```

说明：

- `tool` 必填
- `input` 可选；缺省时会使用空对象
- 当前 tool 执行失败会返回 `400`

## SSE Streams

### Task Stream

`GET /v1/tasks/{task_id}/stream`

响应头：

```http
Content-Type: text/event-stream; charset=utf-8
Cache-Control: no-cache
Connection: keep-alive
```

当前行为规则：

- 连接建立后会先发送一个 `task.snapshot` 事件，表示当前任务状态
- 如果已经存在持久化审计记录，服务端会先回放 action output / completion 事件
- 如果任务已经处于终态（`succeeded` 或 `failed`），流会在 snapshot + replay 后结束
- 否则服务端会继续订阅实时任务事件

当前任务流事件名：

- `task.snapshot`
- `task.created`
- `task.planned`
- `task.action.dispatched`
- `task.action.output`
- `task.action.completed`

### Action Stream

`GET /v1/tasks/{task_id}/actions/{action_id}/stream`

当前行为规则：

- 如果 audit record 已存在，服务端会回放 `task.action.output` 与 `task.action.completed` 后关闭连接
- 否则会订阅实时 `task.action.output` 和 `task.action.completed`
- action stream 会在 `task.action.completed` 后结束

### 事件载荷示例

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

## 错误返回格式

大多数错误使用简单 JSON 包装：

```json
{"error":"message"}
```

常见状态码：

- `200` — 成功
- `400` — 参数错误或非法查询值
- `401` — 缺失 / 非法 Bearer Token
- `403` — tenant 不匹配或跨租户读取被拒绝
- `404` — 任务不存在、agent 不存在，或 replay 来源不存在
- `405` — 方法不允许
- `500` — API、audit store、event bus 或 gateway 依赖未配置

## 下一步阅读

- [核心能力参考](core-capabilities-cn.md)
- [运行时与沙箱参考](runtime-and-sandbox-cn.md)
- [快速上手](../guides/getting-started-cn.md)
- [配置参考](configuration-cn.md)
- [架构概览](../architecture/overview.md)
