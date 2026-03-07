# AgentOS

**AgentOS = AI Agent 的 Kubernetes**

一个开源的 Agent 执行平台，采用 Go 控制面 + Rust 运行时架构。LLM 不再直接调用工具 — 它们运行在 AgentOS 上。

安全。可控。可扩展。分布式。

## 为什么需要 AgentOS

当前的 Agent 框架（LangChain、AutoGPT、CrewAI）本质是应用层胶水代码，存在四个致命缺陷：

| 问题 | AgentOS 解决方案 |
|------|-----------------|
| **不安全** — AI 生成的代码直接在宿主机运行 | Rust 沙箱 + Docker/gVisor 隔离、环境隔离、秘钥脱敏 |
| **不可扩展** — 工具生态碎片化，缺乏统一标准 | 插拔式 Tool 接口、7 个内置工具、OpenClaw 生态兼容 |
| **不可观测** — 执行过程是黑盒 | 事件驱动审计链路、遥测流、策略日志 |
| **非生产就绪** — 只能单机跑 Demo | NATS 分布式调度、多 Worker 池、自动扩缩容 |

## 系统架构

```
            [ 客户端 (CLI / API / UI / SDK) ]
                          |
  +-----------------------v-----------------------+
  |              接入层 (Go)                       |
  |        HTTP 网关 + CLI + 认证                  |
  +-----------------------+-----------------------+
                          |
  +-----------------------v-----------------------+
  |            编排核心 (Go)                       |
  |                                               |
  |  [LLM Planner]  [Task Engine]  [Scheduler]   |
  |  [Skill Resolver]  [Policy Engine]            |
  +-----------------------+-----------------------+
                          |
  +-----------------------v-----------------------+
  |        Worker 池 + 注册中心 (Go)               |
  |  [Registry]  [Health Monitor]  [Pool]         |
  +-----------------------+-----------------------+
                          |
  +-----------------------v-----------------------+
  |          执行 Worker (Rust)                    |
  |                                               |
  |  [RuntimeAdapter]  [SecurityPolicy]           |
  |  [ActionExecutor]  [Registration]             |
  +-----------------------+-----------------------+
                          |
  +-----------------------v-----------------------+
  |             执行沙箱                           |
  |     [Native]    [Docker]    [WASM (未来)]      |
  +-----------+-----------------------------------+
              |
  +-----------v-----------------------------------+
  |             工具生态                           |
  |  shell / file / git / http / browser (未来)    |
  +-----------------------------------------------+
```

## 六大核心系统

| 系统 | 职责 | 状态 |
|------|------|------|
| **Access 接入层** | HTTP API、CLI、网关 | 已实现 |
| **Agent Brain 智脑层** | LLM Planner（OpenAI 兼容）、Agent YAML DSL | 已实现 |
| **Task Engine 任务引擎** | 状态机、生命周期流转 | 已实现 |
| **Skill System 技能系统** | Tool 注册表、7 个内置工具、SchemaAware | 已实现 |
| **Policy Engine 策略引擎** | 允许/拒绝规则、自治级别、凭证隔离 | 已实现 |
| **Runtime 运行时** | Rust Worker、NativeRuntime、DockerRuntime、SecurityPolicy | 已实现 |
| **Scheduler 调度器** | Worker 注册、健康监测、NATS 队列、Worker 池 | 已实现 |
| **Audit 审计存储** | 平台级持久化 task/action 审计记录 | 已实现 |
| **Memory 记忆系统** | 内存 + Redis 提供者、TTL 支持 | 已实现 |

## 快速开始

```bash
# 终端 1：启动 Rust Worker
cd runtime && cargo run -p agentos-worker

# 终端 2：提交任务
export AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051
go run ./cmd/osctl submit "echo hello"
# 输出: task task-xxx created (state: succeeded)
```

当设置 `AGENTOS_WORKER_ADDR` 时，上面的直连 Worker 流程是 `osctl` 和 `apiserver` 当前支持的本地运行方式。此时如果本地调度器没有可用 worker，控制面会回退到该直连 worker 地址执行。

`controller` 仍然用于 worker 注册和健康监测，但它属于另一条多进程运行路径，不是上面这个本地快速开始的必需项。

接入真实 LLM：

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051 \
       AGENTOS_LLM_API_KEY=sk-xxx \
       AGENTOS_LLM_BASE_URL=https://api.openai.com \
       AGENTOS_LLM_MODEL=gpt-4o
go run ./cmd/osctl submit "创建一个 hello world Python 脚本"
```

HTTP 网关当前暴露 `/health`、`/agent/run`、`/agent/status`、`/agent/list`、`/tool/run`、用于任务级 SSE 遥测的 `GET /v1/tasks/{task_id}/stream`、用于动作实时 stdout/stderr 流的 `GET /v1/tasks/{task_id}/actions/{action_id}/stream`、用于任务审计记录查询的 `GET /v1/tasks/{task_id}/audit`，以及用于单个 action 审计记录查询的 `GET /v1/tasks/{task_id}/actions/{action_id}/audit`。

现在任务遥测会包含实时 `task.action.output` chunk 事件；Native 与 Docker runtime 都支持增量流式输出。当 action 完成后，audit store 会持久化最终 command、exit code、worker id、stdout、stderr，因此动作流接口可以回放已完成动作的结果快照，audit 接口也能提供持久化执行记录。

如果要跑真实三进程验收（`controller + apiserver + worker`，并验证 audit 闭环），可直接执行：

```bash
./scripts/acceptance.sh
```

## Agent DSL

Agent 是配置，不是代码：

```yaml
name: defi-trading-agent
description: "监控市场并执行交易"
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

系统解析此 YAML，动态分配 LLM、挂载 Redis 短期记忆、向 Policy Engine 注册权限白名单。

## 内置工具

| 工具 | 说明 |
|------|------|
| `shell` | 执行 Shell 命令（沙箱隔离） |
| `file.read` | 读取文件内容 |
| `file.write` | 写入文件（自动创建目录） |
| `git.clone` | 克隆 Git 仓库 |
| `git.status` | 获取 Git 状态 |
| `http.get` | HTTP GET 请求 |
| `http.post` | HTTP POST 请求 |

## 目录结构

```
agentos/
├── api/
│   ├── proto/agentos/v1/          # Protobuf 契约 (Go <-> Rust)
│   └── gen/                       # 生成的 Go gRPC 代码
├── cmd/
│   ├── apiserver/                 # HTTP + SSE API 服务
│   ├── controller/                # 编排循环
│   ├── claw-cli/                  # ClawOS CLI
│   └── osctl/                     # 开发者 CLI
├── internal/
│   ├── access/                    # HTTP 处理器、CLI 接线、认证
│   ├── adapters/
│   │   ├── llm/openai/            # OpenAI 兼容 LLM 适配器
│   │   ├── memory/{inmemory,redis}/ # 记忆提供者
│   │   ├── messaging/{memory,nats}/ # EventBus 适配器
│   │   ├── persistence/{memory,postgres}/ # TaskRepository 适配器
│   │   └── runtimeclient/         # gRPC 执行客户端
│   ├── agent/                     # Agent YAML DSL、运行时、管理器
│   ├── bootstrap/                 # 基于配置的依赖注入
│   ├── gateway/                   # HTTP API (/agent/run, /tool/run)
│   ├── memory/                    # Memory 接口 + 工厂
│   ├── orchestration/             # TaskEngine、Planner、状态机
│   ├── policy/                    # PolicyEngine、规则、凭证金库
│   ├── scheduler/                 # 本地 + NATS 任务调度器
│   ├── tool/builtin/              # 7 个内置工具
│   └── worker/                    # 注册中心、健康监测、连接池
├── pkg/
│   ├── config/                    # 全系统配置
│   ├── events/                    # 领域事件
│   └── taskdsl/                   # Task、Plan、Action 类型
├── runtime/
│   └── crates/
│       ├── worker/                # Rust gRPC Worker + 执行器
│       ├── sandbox/               # RuntimeAdapter、NativeRuntime、DockerRuntime
│       └── telemetry/             # 流式遥测模型
├── deploy/                        # Docker Compose (NATS + Postgres)
└── examples/
    ├── agents/                    # Agent YAML 示例
    └── basic-task/
```

## 可插拔适配器

| 接口 | 适配器 | 默认值 |
|------|--------|--------|
| `EventBus` | memory, nats | nats（生产）/ memory（开发） |
| `TaskRepository` | memory, postgres | postgres（生产）/ memory（开发） |
| `AuditLogStore` | memory, postgres | postgres（生产）/ memory（开发） |
| `Planner` | prompt, openai (LLM) | 默认 prompt 回退；配置 LLM 后启用 OpenAI + retry/fallback 流水线 |
| `Memory.Provider` | inmemory, redis | inmemory |
| `RuntimeAdapter` (Rust) | native, docker | native |
| `Scheduler` | local, nats | nats（生产）/ local（开发） |

```go
app, err := bootstrap.FromEnv(ctx)
// AGENTOS_MODE=dev  -> 内存适配器 + 本地调度器 + prompt planner
// AGENTOS_MODE=prod -> nats + postgres + nats 调度 + 配置 API key 时启用 OpenAI planner
```

## 安全模型

**Go 控制面（借鉴 HiClaw）：**
- PolicyEngine：基于 glob 的允许/拒绝规则，拒绝优先
- 自治级别：Supervised（监督）/ SemiAutonomous（半自治）/ Autonomous（全自治）
- 凭证金库：Worker 只拿不透明 token，真实密钥仅在网关解析
- 按 Agent 粒度限流（actions/hour）
- 危险命令黑名单（rm -rf、dd、mkfs 等）

**Rust Worker（借鉴 ZeroClaw）：**
- SecurityPolicy：命令白名单/黑名单、禁止路径
- 环境隔离：清空所有环境变量，仅保留安全变量（PATH、HOME、TERM）
- 秘钥脱敏：自动检测 API Key、Bearer Token、AWS 密钥并脱敏
- 输出截断：可配置最大字节数（默认 1MB）
- 超时强制执行
- Docker 隔离：--read-only、--network none、资源限制

## 分布式架构

```
                    ┌─────────────────┐
                    │   Go 控制面      │
                    │                 │
                    │  ┌───────────┐  │
                    │  │  调度器    │  │
                    │  └─────┬─────┘  │
                    │        │        │
                    │  ┌─────▼─────┐  │
                    │  │  Worker   │  │
                    │  │  注册中心  │  │
                    │  └─────┬─────┘  │
                    └────────┼────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼─────┐
        │  Worker 1  │ │  Worker 2  │ │  Worker N  │
        │  (Rust)    │ │  (Rust)    │ │  (Rust)    │
        │ native/    │ │ docker/    │ │ docker/    │
        │ docker     │ │ native     │ │ wasm       │
        └────────────┘ └────────────┘ └────────────┘
```

- Worker 启动时向控制面注册
- 周期性心跳（默认 10s），离线检测（30s 超时）
- 生产调度路径使用 NATS dispatch/result subject，并由 dispatcher bridge 执行
- 开发模式仍可通过 `AGENTOS_WORKER_ADDR` 直连 worker
- Worker 池结合最少负载选择与懒加载 gRPC 连接缓存

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `AGENTOS_MODE` | dev（内存 + local 调度）或 prod（nats + postgres） | prod |
| `AGENTOS_WORKER_ADDR` | 开发/直连执行时使用的 Rust Worker gRPC 地址 | localhost:50051 |
| `AGENTOS_CONTROL_PLANE_ADDR` | 共享 controller 注册中心地址，用于远程 worker 发现 | — |
| `AGENTOS_SCHEDULER_MODE` | `local` 或 `nats` | 生产：`nats`，开发：`local` |
| `AGENTOS_API_LISTEN_ADDR` | API 服务监听地址 | :8080 |
| `AGENTOS_LLM_API_KEY` | LLM API 密钥（启用 OpenAI planner 流水线） | — |
| `AGENTOS_LLM_BASE_URL` | LLM API 基础 URL | https://api.openai.com |
| `AGENTOS_LLM_MODEL` | LLM 模型名 | gpt-4o |
| `AGENTOS_RUNTIME` | Worker 运行时：native 或 docker | native |
| `AGENTOS_SECURITY_LEVEL` | supervised / semi / autonomous | supervised |
| `AGENTOS_DOCKER_IMAGE` | Docker 沙箱镜像 | ubuntu:22.04 |
| `AGENTOS_MAX_CONCURRENT_TASKS` | Worker 并发上限 | 4 |
| `AGENTOS_AGENT_SECRETS` | Agent 密钥映射（`agent=secret,agent2=secret2`），用于注入不透明 token | — |

## 测试

```bash
# Go 测试（13 个测试套件）
go test ./...

# Rust 测试（5 个测试套件，69+ 个测试）
cd runtime && cargo test --workspace
```

## 演进路线

| 阶段 | 重点 | 状态 |
|------|------|------|
| Stage 1: MVP | 核心链路（提交 → 规划 → 执行 → 结果） | 已完成 |
| Stage 2: 智能体系 | LLM planner 流水线、Agent YAML DSL、工具、记忆 | 已完成 |
| Stage 3: 沙箱与安全 | Docker 隔离、SecurityPolicy、PolicyEngine | 已完成 |
| Stage 4: 分布式调度 | Worker 注册、NATS 调度、Worker 池 | 已完成 |
| Stage 5: 商业化平台 | Web UI (Claw Studio)、SDK、Agent 市场 | 规划中 |

## 文档

- [架构概览](docs/architecture/overview.md)
- [AgentOS v1 架构](docs/architecture/agentos-v1-architecture.md)
- [ClawOS v1 架构](docs/architecture/clawos-v1-architecture.md)
- [Monorepo 结构](docs/architecture/monorepo-structure.md)
- [可插拔适配器](docs/architecture/adapters.md)
- [技能系统](docs/architecture/skill-system.md)
- [策略引擎](docs/architecture/policy-engine.md)
- [MVP 范围](docs/architecture/mvp-scope.md)

## 贡献

参见 [CONTRIBUTING.md](CONTRIBUTING.md) 了解开发环境搭建和贡献指南。

## 许可证

AgentOS 是开源项目。具体许可证信息请查看各文件。
