# AgentOS

采用 Go 控制面与 Rust 运行时平面的开源 Agent 执行平台。

AgentOS 的目标，是让团队把 Agent 当作可治理工作负载来运行，而不是停留在临时脚本和 prompt demo。

## AgentOS 是什么

当前公开仓库聚焦在 **Community 核心**：

- 可 self-host 的控制面与 worker runtime
- 任务编排与执行生命周期
- 本地调度与 NATS 调度路径
- audit、replay 与 SSE telemetry API
- agent loop / tool-calling 能力
- tools、skills、adapters 等扩展接口

## 适合谁

AgentOS 当前更适合以下团队：

- 构建内部 Agent 平台的平台 / 基础设施团队
- 运行研发自动化或开发者工具链的工程效率团队
- 对审计、调度、执行控制有要求的运维与流程自动化团队
- 想要 self-host 执行底座，而不是聊天外壳的开发者

当前项目 **并不** 以“完整终端聊天产品”或“成熟企业控制台”作为主要定位。

## 为什么需要 AgentOS

很多 Agent 项目擅长 prompt 和 demo，但在执行基础设施层仍然薄弱。

| 需求 | AgentOS 提供什么 |
|------|------------------|
| 安全执行 | Rust worker runtime、沙箱路径、policy hook、secret 隔离 |
| 运维控制 | task lifecycle、调度、worker registry、可回放执行记录 |
| 可观测性 | audit trail、SSE telemetry、action output 流 |
| 可扩展性 | tools、skills、adapters、control-plane bridge |
| 自部署能力 | 可在自有环境中运行的 community core |

## 快速开始

按你的目标选择一条路径即可。

### 1. 最快的本地验证路径

```bash
# Terminal 1: 启动 Rust worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: 通过本地控制路径提交任务
export AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051
go run ./cmd/osctl submit "echo hello"
```

这条路径最适合快速确认执行底座本身能跑通。

### 2. 开启 LLM 规划的本地路径

```bash
export AGENTOS_MODE=dev \
       AGENTOS_WORKER_ADDR=localhost:50051 \
       AGENTOS_LLM_PROVIDER=openai \
       AGENTOS_LLM_API_KEY=sk-xxx \
       AGENTOS_LLM_BASE_URL=https://api.openai.com \
       AGENTOS_LLM_MODEL=gpt-4o
go run ./cmd/osctl submit "create a hello world python script"
```

这条路径会在相同执行底座之上启用 LLM planner / agent loop 能力。

### 3. 完整多进程验收

```bash
./scripts/acceptance.sh
```

这会验证真实的 `controller + apiserver + worker` 链路，包括鉴权、调度、审计、回放，以及 control-plane bridge 行为。

## 架构概览

```text
Clients (CLI / API / SDK)
  -> Access Layer (Go)
  -> Orchestration Core (Go)
  -> Scheduler / Worker Registry (Go)
  -> Execution Workers (Rust)
  -> Sandbox / Tool Surfaces
```

高层看，系统由几部分组成：

- `apiserver` 提供 HTTP、audit、replay 与 SSE telemetry API
- `controller` 负责共享 worker 注册和控制面协调
- orchestration core 管理 task state、planning、policy 与 dispatch
- workers 通过 native 或 container-backed runtime 执行动作
- 部分 tool-like action 也可以直接通过 Go 控制面的 bridge 执行

## 核心系统

| 系统 | 职责 | 状态 |
|------|------|------|
| **Access** | HTTP API、CLI、Gateway | 已实现 |
| **Agent Brain** | 注册表驱动的 LLM Planner（OpenAI-compatible）、Agent YAML DSL | 已实现 |
| **Task Engine** | 状态机、生命周期迁移 | 已实现 |
| **Skill System** | Tool registry、内置工具、SchemaAware、file/http 类 action bridge | 已实现 |
| **Policy Engine** | allow/deny 规则、自主级别、凭证隔离 | 已实现 |
| **Runtime** | Rust Worker、NativeRuntime、DockerRuntime、SecurityPolicy | 已实现 |
| **Scheduler** | Worker registry、health monitor、NATS queue、worker pool | 已实现 |
| **Audit** | 平台级 audit store、持久化 task/action 记录 | 已实现 |
| **Memory** | In-memory + Redis provider、TTL 支持 | 已实现 |

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
| `Planner` | prompt、基于注册表的 LLM provider（内置 `openai`） | 默认 prompt planner；LLM planner 对瞬时错误做有限重试，对非法 JSON 先做一次修复，再回退到 `PromptPlanner` |
| `Memory.Provider` | inmemory, redis | inmemory |
| `RuntimeAdapter` (Rust) | native, docker | native |
| `Scheduler` | local, nats | nats（生产）/ local（开发） |

```go
app, err := bootstrap.FromEnv(ctx)
// AGENTOS_MODE=dev  -> 内存适配器 + 本地调度器 + prompt planner
// AGENTOS_MODE=prod -> nats + postgres + nats 调度 + 按配置启用的注册表式 LLM planner
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
| `AGENTOS_NATS_URL` | 消息/调度适配器使用的 NATS 服务地址 | nats://localhost:4222 |
| `AGENTOS_NATS_STREAM` | JetStream stream 名称 | AGENTOS |
| `AGENTOS_POSTGRES_DSN` | 持久化适配器使用的 PostgreSQL DSN | postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable |
| `AGENTOS_API_LISTEN_ADDR` | API 服务监听地址 | :8080 |
| `AGENTOS_LLM_PROVIDER` | 从 LLM 适配器注册表解析的 planner provider 名称 | 开发模式有 API key 时默认为 openai，否则为 stub |
| `AGENTOS_LLM_API_KEY` | LLM API 密钥（启用当前配置的 planner provider） | — |
| `AGENTOS_LLM_BASE_URL` | OpenAI 兼容 provider 的 LLM API 基础 URL | https://api.openai.com |
| `AGENTOS_LLM_MODEL` | LLM 模型名 | gpt-4o |
| `AGENTOS_RUNTIME` | Worker 运行时：native 或 docker | native |
| `AGENTOS_SECURITY_LEVEL` | supervised / semi / autonomous | supervised |
| `AGENTOS_DOCKER_IMAGE` | Docker 沙箱镜像 | ubuntu:22.04 |
| `AGENTOS_MAX_CONCURRENT_TASKS` | Worker 并发上限 | 4 |
| `AGENTOS_AGENT_SECRETS` | Agent 密钥映射（`agent=secret,agent2=secret2`），用于注入不透明 token | — |
| `AGENTOS_AUTH_TOKENS` | Bearer 鉴权映射（`token=subject|tenant|role1;role2`），保护 `/v1/tasks*` 与 gateway 路由 | — |

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
| Stage 5: 平台体验与生态 | 控制台、SDK、扩展能力 | 规划中 |

## Open Core 边界

AgentOS 将仓库核心以 **Apache-2.0** 方式公开，并把企业增强和托管云能力放在仓库核心边界之外。

- **Community**：开源、自部署的控制面、运行时、调度、audit 与 telemetry API
- **Enterprise**：组织治理、SSO / SCIM / RBAC、审计中心、长周期保留、支持服务
- **Cloud**：托管控制面、控制台、升级运维、计费与 SLA

对外公开的仓库重点是 **Community** 核心：可自部署的执行底座、编排契约、调度、审计与遥测能力。许可证与边界说明见 `docs/strategy/licensing-decision.md`。

## 文档

- [文档导航](docs/README_CN.md)
- [变更记录](CHANGELOG.md)
- [架构概览](docs/architecture/overview.md)
- [多进程验收说明](docs/architecture/multiprocess-acceptance.md)
- [许可证决策](docs/strategy/licensing-decision.md)
- [平台层与能力层边界](docs/architecture/platform-vs-capability-boundary.md)

## 贡献

参见 [CONTRIBUTING.md](CONTRIBUTING.md) 了解开发环境搭建和贡献指南，参见 [SUPPORT.md](SUPPORT.md) 了解社区支持边界，参见 [SECURITY.md](SECURITY.md) 了解漏洞提交流程，参见 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) 了解社区行为规范。

## 许可证

AgentOS 核心仓库采用 [Apache-2.0](LICENSE) 许可证。企业增强模块与托管云服务可以采用独立的商业条款。
