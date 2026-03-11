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

### 1. 一键初始化开发环境

```bash
make dev-setup
source .env.agentos.dev
make dev-up
```

如果你想用最短路径拿到可重复的本地开发环境，这条路径会帮你完成二进制构建、生成可 `source` 的开发环境文件，并拉起完整本地栈。

也可以直接运行底层脚本：

```bash
bash scripts/setup_dev_env.sh --verify-stack
bash scripts/start_full_stack.sh --smoke-test --exit-after-smoke
```

### 2. 最快的本地验证路径

```bash
# Terminal 1: 启动 Rust worker
cd runtime && cargo run -p agentos-worker

# Terminal 2: 通过本地控制路径提交任务
export AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051
go run ./cmd/osctl submit "echo hello"
```

这条路径最适合快速确认执行底座本身能跑通。

### 3. 开启 LLM 规划的本地路径

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

### 4. 完整多进程验收

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

## 延伸阅读

如果你想看首页之外的细节，建议从这里继续：

- [快速上手](docs/guides/getting-started-cn.md)
- [核心能力参考](docs/reference/core-capabilities-cn.md)
- [API 能力面参考](docs/reference/api-surfaces-cn.md)
- [配置参考](docs/reference/configuration-cn.md)
- [运行时与沙箱参考](docs/reference/runtime-and-sandbox-cn.md)
- [架构概览](docs/architecture/overview.md)
- [多进程验收说明](docs/architecture/multiprocess-acceptance.md)
- [文档导航](docs/README_CN.md)
- [变更记录](CHANGELOG.md)

## Open Core 边界

AgentOS 以 `Apache-2.0` 发布平台核心，并把商业化包装放在仓库边界之外。

- **Community** — 可自部署的控制面、worker runtime、调度、audit API、replay、telemetry 与 agent-loop 底座
- **Enterprise（未来）** — 组织治理、SSO / SCIM / RBAC、长周期审计中心与支持流程
- **Cloud（未来）** — 托管控制面、运维控制台、升级、计费与 SLA 能力

当前边界说明见 [许可证决策](docs/strategy/licensing-decision.md) 与 [平台层与能力层边界](docs/architecture/platform-vs-capability-boundary.md)。

## 贡献

参见 [CONTRIBUTING.md](CONTRIBUTING.md) 了解开发环境搭建和贡献指南，参见 [SUPPORT.md](SUPPORT.md)、[SECURITY.md](SECURITY.md) 与 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) 了解社区支持、安全提交流程和行为规范。

## 许可证

开源核心采用 [Apache-2.0](LICENSE) 许可证。企业增强模块与托管云服务可以采用独立的商业条款。
