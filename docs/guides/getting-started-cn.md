# 快速上手

这份文档是在首页 Quick Start 之上的扩展说明，帮助你按目标选择合适的本地运行路径。

## 选择一条本地运行路径

### 1. 最快验证执行底座

适合先确认本地执行链路是否打通，再决定是否开启规划或完整控制面拓扑。

```bash
# Terminal 1: 启动 Rust worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: 通过本地控制路径提交任务
export AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051
go run ./cmd/osctl submit "echo hello"
```

### 2. 开启 LLM 规划的本地路径

适合在同一套执行底座之上启用 planner / agent-loop 行为。

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051 \
       AGENTOS_LLM_PROVIDER=openai \
       AGENTOS_LLM_API_KEY=sk-xxx \
       AGENTOS_LLM_BASE_URL=https://api.openai.com \
       AGENTOS_LLM_MODEL=gpt-4o
go run ./cmd/osctl submit "create a hello world python script"
```

### 3. 完整多进程验收

适合验证真实的 `controller + apiserver + worker` 链路，包括调度、鉴权、审计、回放，以及 control-plane bridge。

```bash
./scripts/acceptance.sh
```

## 关键环境变量

| 变量 | 用途 | 默认值 |
|------|------|--------|
| `AGENTOS_MODE` | `dev` 使用内存适配器和本地调度；`prod` 走 NATS + Postgres 路径 | `prod` |
| `AGENTOS_WORKER_ADDR` | 本地 / 开发直连执行时使用的 Rust worker gRPC 地址 | `localhost:50051` |
| `AGENTOS_CONTROL_PLANE_ADDR` | 共享 controller 注册中心地址，用于远程 worker 发现 | — |
| `AGENTOS_SCHEDULER_MODE` | `local` 或 `nats` | 生产：`nats`，开发：`local` |
| `AGENTOS_NATS_URL` | 消息与调度使用的 NATS 地址 | `nats://localhost:4222` |
| `AGENTOS_NATS_STREAM` | JetStream stream 名称 | `AGENTOS` |
| `AGENTOS_POSTGRES_DSN` | 持久化适配器使用的 Postgres DSN | `postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable` |
| `AGENTOS_API_LISTEN_ADDR` | API 监听地址 | `:8080` |
| `AGENTOS_LLM_PROVIDER` | 从适配器注册表解析的 planner provider 名称 | 开发模式配置后默认为 `openai`，否则回退 stub |
| `AGENTOS_LLM_API_KEY` | LLM provider API key | — |
| `AGENTOS_LLM_BASE_URL` | OpenAI-compatible base URL | `https://api.openai.com` |
| `AGENTOS_LLM_MODEL` | LLM 模型名 | `gpt-4o` |
| `AGENTOS_RUNTIME` | Worker 运行时适配器 | `native` |
| `AGENTOS_SECURITY_LEVEL` | `supervised`、`semi` 或 `autonomous` | `supervised` |
| `AGENTOS_DOCKER_IMAGE` | Docker 沙箱镜像 | `ubuntu:22.04` |
| `AGENTOS_MAX_CONCURRENT_TASKS` | Worker 并发上限 | `4` |
| `AGENTOS_AGENT_SECRETS` | Agent 密钥映射，用于注入不透明 token | — |
| `AGENTOS_AUTH_TOKENS` | `/v1/tasks*` 与 gateway 路由使用的 Bearer 鉴权映射 | — |

## 如何验证整套系统

```bash
# Go 测试
go test ./...

# Rust 测试
cd runtime && cargo test --workspace

# 真实多进程验收
./scripts/acceptance.sh
```

## 下一步阅读

- [核心能力参考](../reference/core-capabilities-cn.md)
- [配置参考](../reference/configuration-cn.md)
- [架构概览](../architecture/overview.md)
- [多进程验收说明](../architecture/multiprocess-acceptance.md)
- [文档导航](../README_CN.md)
