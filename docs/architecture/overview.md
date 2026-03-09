# AgentOS Architecture Overview

AgentOS 是一个采用 Go 控制面与 Rust 运行时平面的开源 Agent 执行平台。

这份文档描述的是**当前公开仓库的主线形态**，也就是社区可用的 self-hosted execution core，而不是一个面向终端用户的聊天产品说明页。

如果你是第一次阅读这个仓库，建议按以下顺序继续：

1. [Project README](../../README.md)
2. [Getting Started](../guides/getting-started.md)
3. [Core Capabilities Reference](../reference/core-capabilities.md)
4. [Concept Docs Index](../concepts/README.md)

## 当前定位

- **是什么**：用于提交、规划、调度、执行、审计与回放 agent work 的执行平台
- **公开仓库提供什么**：community core，包括控制面、worker runtime、调度、audit / replay API、SSE telemetry、tool bridge
- **不是什么**：当前并不以完整聊天产品、成熟企业控制台或托管云服务作为公开仓库主目标

## 高层系统形态

```text
Clients (CLI / API / SDK)
  -> Access Layer (Go)
  -> Orchestration Core (Go)
  -> Policy / Scheduler / Worker Registry (Go)
  -> Execution Workers (Rust)
  -> Sandbox / Tool Surfaces
  -> Audit / Replay / Telemetry
```

## 核心层次

| 层 | 组件 | 作用 |
|----|------|------|
| Access Layer | `apiserver`、CLI、gateway routes | 接收任务、暴露查询与流式接口、处理鉴权 |
| Orchestration Core | planner、task engine、state machine | 把 prompt / task 转换成 actions，并驱动生命周期 |
| Control Plane | policy engine、scheduler、worker registry、pool | 做规则拦截、worker 选择、dispatch 与结果处理 |
| Runtime Plane | Rust worker、native runtime、docker runtime | 真正执行 action，并返回结构化结果与流式输出 |
| Persistence & Messaging | task repo、audit store、event bus、NATS / Postgres adapters | 提供持久化、事件分发与分布式调度基础 |
| Observability | audit APIs、replay APIs、SSE task/action streams | 提供任务与动作级别的可观测性 |

## 端到端请求流

典型任务请求会经过以下路径：

1. 客户端向 `POST /v1/tasks` 提交任务
2. `apiserver` 做鉴权并进入 orchestration core
3. planner 生成 plan / actions，必要时结合 agent profile 与 memory context
4. policy engine 在每个 action 执行前做 allow / deny、危险命令、自治级别等检查
5. scheduler 通过本地或 NATS 路径把 action 派发到 worker pool
6. Rust worker 在 native 或 docker runtime 中执行，并持续回传输出 chunk / 最终结果
7. 控制面把结果写入 task repository 与 audit store，并发布领域事件
8. API 层对外暴露状态查询、audit、replay，以及 task / action 级 SSE telemetry

## 当前实现重点

### 1. 多进程控制面闭环

- `controller` 负责共享 worker 注册与协调
- `apiserver` 可以通过共享 registry 发现远程 worker
- `agentos-worker` 通过 gRPC 注册、心跳并接收调度
- 真实三进程验收由 [`./scripts/acceptance.sh`](../../scripts/acceptance.sh) 覆盖

### 2. 双调度路径

- **dev / local path**：适合单机直连 worker 的本地开发与快速验证
- **NATS-backed path**：适合多 worker、异步 dispatch 与结果回传

### 3. Planner / Tool / Memory 集成

- 支持 prompt baseline 与 registry-backed LLM planner
- 支持 tool-like actions 通过 Go control-plane bridge 执行
- 支持 in-memory / Redis memory providers
- 支持 credential-vault 风格的 agent secret 注入路径

### 4. 审计与遥测

- `GET /v1/audit` 提供平台级 audit feed
- `GET /v1/tasks/{task_id}/audit` 与 action audit 提供持久化执行记录
- `GET /v1/tasks/{task_id}/replay` 提供任务回放
- `GET /v1/tasks/{task_id}/stream` 与 action stream 提供 SSE telemetry

## 当前仓库的阅读方式

不同文档适合不同目的：

- [Getting Started](../guides/getting-started.md) —— 快速跑通本地路径、LLM 路径、多进程 acceptance
- [Core Capabilities Reference](../reference/core-capabilities.md) —— 查看 DSL、工具、适配器、安全模型、仓库结构
- [Multiprocess Acceptance](multiprocess-acceptance.md) —— 理解真实三进程验收闭环
- [Concept Docs Index](../concepts/README.md) —— 阅读更长的历史设计与目标态背景说明

## 设计原则

- **控制面与运行时解耦**：Go 负责编排，Rust 负责执行
- **接口优先**：planner、repo、bus、scheduler、runtime client 都通过接口装配
- **多适配器**：memory / redis、memory / nats、memory / postgres、native / docker 可切换
- **先治理再执行**：policy、audit、replay、telemetry 是平台主线，而不是附属能力
- **开源核心聚焦执行底座**：先把 self-hosted execution core 做扎实，再向更高层产品形态延展
