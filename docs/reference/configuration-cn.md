# 配置参考

这份文档说明 AgentOS 当前的配置面。

它聚焦的是公开二进制当前实际使用的配置路径：

- `bootstrap.FromEnv(ctx)`
- `config.Default()` 或 `config.Dev()`
- `config.ApplyEnvOverrides(...)`

## 配置模型

当前主启动路径以**环境变量驱动**为主。

启动时会按以下顺序决策：

1. 如果 `AGENTOS_MODE=dev`，从 `config.Dev()` 开始
2. 否则从 `config.Default()` 开始
3. 再通过 `config.ApplyEnvOverrides(...)` 叠加环境变量覆盖

这也意味着当前公开二进制的主路径**不会**加载一个通用的平台 YAML 配置文件。

## 模式基线

### 生产基线

`config.Default()` 的默认值：

| 区域 | 默认值 |
|------|--------|
| worker 地址 | `localhost:50051` |
| messaging provider | `nats` |
| NATS URL | `nats://localhost:4222` |
| NATS stream | `AGENTOS` |
| persistence provider | `postgres` |
| Postgres DSN | `postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable` |
| LLM provider | `openai` |
| LLM model | `gpt-4o` |
| LLM base URL | `https://api.openai.com` |
| memory provider | `inmemory` |
| memory TTL | `24h` |
| 默认 autonomy | `supervised` |
| scheduler mode | `nats` |
| scheduler heartbeat timeout | `30s` |
| scheduler health check interval | `10s` |
| agent dir | `agents` |

### 开发基线

`config.Dev()` 的默认值：

| 区域 | 默认值 |
|------|--------|
| worker 地址 | 如果设置了 `AGENTOS_WORKER_ADDR` 就取它 |
| messaging provider | `memory` |
| persistence provider | `memory` |
| LLM provider | 默认 `stub`；如果存在 `AGENTOS_LLM_API_KEY` 会自动切到 `openai` |
| LLM model | `gpt-4o` |
| LLM base URL | `https://api.openai.com` |
| memory provider | `inmemory` |
| 默认 autonomy | `autonomous` |
| scheduler mode | `local` |
| scheduler heartbeat timeout | `30s` |
| scheduler health check interval | `10s` |

## Planner 选择说明

当前 planner 行为同时受 provider 支持情况和 API key 是否存在影响：

- 如果没有可用的 LLM provider，bootstrap 会回退到 `PromptPlanner`
- 如果配置了 `openai` 但没有 API key，也会回退到 `PromptPlanner`
- 如果存在受支持的 provider 且有 API key，bootstrap 会使用带 retry + fallback 的 LLM planner
- 不支持的 provider 名称同样会回退到 `PromptPlanner`

## 环境变量

### 控制面启动变量

这些变量由 `pkg/config` 和 `internal/bootstrap` 读取。

| 变量 | 用途 | 默认值 / 行为 |
|------|------|---------------|
| `AGENTOS_MODE` | 选择启动基线 | 未设置 = 生产基线，`dev` = 开发基线 |
| `AGENTOS_WORKER_ADDR` | 直连 Rust worker 的 gRPC 地址 | 生产默认 `localhost:50051`；开发模式下用于本地 worker 路径 |
| `AGENTOS_CONTROL_PLANE_ADDR` | 共享 controller 注册中心地址 | 默认未设置 |
| `AGENTOS_SCHEDULER_MODE` | 覆盖 scheduler 模式 | 基线默认决定 `nats` 或 `local` |
| `AGENTOS_NATS_URL` | NATS 连接地址 | `nats://localhost:4222` |
| `AGENTOS_NATS_STREAM` | JetStream stream 名称 | `AGENTOS` |
| `AGENTOS_POSTGRES_DSN` | Postgres DSN | `postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable` |
| `AGENTOS_LLM_PROVIDER` | planner provider 名称 | 生产基线为 `openai`；开发基线默认为 `stub`，有 API key 时自动切 `openai` |
| `AGENTOS_LLM_API_KEY` | LLM API key | 默认未设置 |
| `AGENTOS_LLM_BASE_URL` | OpenAI-compatible base URL | `https://api.openai.com` |
| `AGENTOS_LLM_MODEL` | LLM 模型名 | `gpt-4o` |
| `AGENTOS_AGENT_SECRETS` | agent secret 映射 | 解析格式为 `agent=secret,agent2=secret2` |
| `AGENTOS_AUTH_TOKENS` | bearer token 映射 | 解析格式为 `token=subject\|tenant\|role1;role2,...` |

### HTTP API Server

| 变量 | 用途 | 默认值 |
|------|------|--------|
| `AGENTOS_API_LISTEN_ADDR` | `apiserver` 监听地址 | `:8080` |

### CLI 变量

| 变量 | 消费方 | 用途 | 默认值 |
|------|--------|------|--------|
| `AGENTOS_SERVER_URL` | `osctl` | 远程 AgentOS API 基础地址 | 空 = 使用本地嵌入模式 |
| `AGENTOS_AUTH_TOKEN` | `osctl`、`claw-cli` | 已鉴权服务使用的 bearer token | 未设置 |

### Controller

| 变量 | 用途 | 默认值 |
|------|------|--------|
| `GRPC_LISTEN_ADDR` | controller 的 worker registry gRPC 监听地址 | `:50052` |

### Rust Worker Runtime

这些变量由 `runtime/crates/worker/src/config.rs` 读取。

| 变量 | 用途 | 默认值 |
|------|------|--------|
| `AGENTOS_LISTEN_ADDR` | worker gRPC 监听地址 | `127.0.0.1:50051` |
| `AGENTOS_WORKER_ID` | 唯一 worker id | `hostname-random` 后缀 |
| `AGENTOS_CONTROL_PLANE_ADDR` | 用于注册的 controller 地址 | 未设置 |
| `AGENTOS_HEARTBEAT_INTERVAL_SECS` | 心跳间隔（秒） | `10` |
| `AGENTOS_MAX_CONCURRENT_TASKS` | worker 并发上限 | `4` |
| `AGENTOS_RUNTIME` | runtime 类型 | `native` |
| `AGENTOS_DOCKER_IMAGE` | docker runtime 镜像 | `ubuntu:22.04` |
| `AGENTOS_DOCKER_MEMORY_MB` | docker 内存限制 | `512` |
| `AGENTOS_DOCKER_CPU_LIMIT` | docker CPU 限制 | `1.0` |
| `AGENTOS_DOCKER_NETWORK` | docker 网络模式 | `none` |
| `AGENTOS_DOCKER_MOUNT_WORKSPACE` | 是否挂载宿主 workspace | `false` |
| `AGENTOS_DOCKER_READ_ONLY` | 是否使用只读 rootfs | `false` |
| `AGENTOS_SECURITY_LEVEL` | autonomy 模式 | `supervised` |
| `AGENTOS_MAX_ACTIONS_PER_HOUR` | action 频率限制 | `120` |
| `AGENTOS_MAX_OUTPUT_BYTES` | 输出截断上限 | `1048576` |
| `AGENTOS_FORBIDDEN_PATHS` | 逗号分隔的禁用路径 | 使用 security policy 默认值 |

### 测试 / 集成变量

这些变量主要用于测试与本地验证，不属于正常生产启动面：

| 变量 | 用途 |
|------|------|
| `AGENTOS_TEST_POSTGRES_DSN` | postgres 集成测试 |
| `AGENTOS_REDIS_ADDR` | redis 集成测试 |

## 编码格式说明

### `AGENTOS_AGENT_SECRETS`

格式：

```text
agent-a=secret-a,agent-b=secret-b
```

含义：

- 每个条目格式为 `agent_name=secret`
- 非法或空条目会被忽略
- 这些值会进入控制面的 credential vault 映射

### `AGENTOS_AUTH_TOKENS`

格式：

```text
token-a=user-a|tenant-a|admin;writer,token-b=user-b|tenant-b
```

含义：

- 每个条目格式为 `token=subject|tenant|role1;role2`
- `subject` 必填
- `tenant` 与 `roles` 可选
- 多个 role 用 `;` 分隔

## 实用配置画像

### 最快的本地开发

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051
```

这会得到：

- memory messaging
- memory persistence
- local scheduler
- 不依赖外部 NATS / Postgres

### 开启 LLM 的本地开发

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051 \
       AGENTOS_LLM_PROVIDER=openai \
       AGENTOS_LLM_API_KEY=sk-xxx \
       AGENTOS_LLM_BASE_URL=https://api.openai.com \
       AGENTOS_LLM_MODEL=gpt-4o
```

### 本地多进程控制面

```bash
export AGENTOS_API_LISTEN_ADDR=127.0.0.1:18080
export GRPC_LISTEN_ADDR=127.0.0.1:15052
export AGENTOS_CONTROL_PLANE_ADDR=127.0.0.1:15052
```

### Docker-backed worker

```bash
export AGENTOS_RUNTIME=docker \
       AGENTOS_DOCKER_IMAGE=ubuntu:22.04 \
       AGENTOS_DOCKER_NETWORK=none \
       AGENTOS_DOCKER_MEMORY_MB=512 \
       AGENTOS_DOCKER_CPU_LIMIT=1.0
```

## 目前还不是稳定环境变量面的部分

`Config` 结构里还有更多字段，但当前 `FromEnv` 主路径并没有把它们全部暴露成一等环境变量。

例如：

- 在公共 env surface 上切换 memory provider
- 通过 env 调整 memory TTL / Redis 地址 / Redis 前缀
- 通过 env 注入 policy rule 列表
- 通过 env 切换 agent 目录
- 通过 env 覆盖 scheduler timing

这些能力目前更适合在代码里直接构造 `bootstrap.New(ctx, cfg)`，而不是只依赖 `bootstrap.FromEnv(ctx)`。

## 下一步阅读

- [API 能力面参考](api-surfaces-cn.md)
- [核心能力参考](core-capabilities-cn.md)
- [快速上手](../guides/getting-started-cn.md)
