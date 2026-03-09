# AgentOS v1 Architecture

> **状态说明**：这是一份较长的 v1 设计参考文档，包含目标态设计与部分历史思路；当前公开仓库的实时实现口径，请优先阅读 [Architecture Overview](../architecture/overview.md)、[Getting Started](../guides/getting-started.md) 与 [Core Capabilities Reference](../reference/core-capabilities.md)。
>
> **当前定位**：AgentOS 是一个采用 Go 控制面与 Rust 运行时平面的开源 Agent 执行平台，公开仓库聚焦 community core，而不是完整终端聊天产品或成熟企业控制台。

## 阅读前说明

- 这份文档适合看“为什么这样分层、目标态如何演进”
- 这份文档**不是**当前 live API 的唯一权威来源
- 遇到接口、能力边界、验收路径时，以 `README`、`docs/guides/`、`docs/reference/` 与 acceptance 文档为准

## 对标

| 系统 | 职责 |
|------|------|
| Kubernetes | container orchestration |
| Ray | distributed compute |
| LangChain | agent SDK / LLM workflow |
| **AgentOS** | **Agent Execution Platform** |

## AgentOS vs 市面框架

| 框架 | 类型 |
|------|------|
| LangChain | LLM workflow |
| AutoGPT | agent demo |
| CrewAI | agent orchestration |
| OpenClaw | computer use |
| HiClaw | CLI agent |
| **AgentOS** | **Agent Execution Platform** |

---

## 总体架构（设计目标态）

> 说明：本节描述的是 v1 设计目标态，不等同于当前公开仓库已经全部实现的能力面。

```
                +-----------------------+
                |        Clients        |
                | CLI / API / UI / SDK  |
                +-----------+-----------+
                            |
                            v
                    +---------------+
                    |  Access Layer  |
                    | API / Auth     |
                    +-------+-------+
                            |
                            v
                    +---------------+
                    |  Agent Brain   |
                    | Planner        |
                    | Reasoner       |
                    +-------+-------+
                            |
                            v
                    +---------------+
                    |  Task Engine   |
                    | StateMachine   |
                    | Retry / Queue  |
                    +-------+-------+
                            |
                            v
                   +------------------+
                   | Skill System     |
                   | Skill Registry   |
                   | Skill Resolver   |
                   +--------+---------+
                            |
                            v
                 +----------------------+
                 | Policy / Guardrail   |
                 | Security Engine      |
                 +----------+-----------+
                            |
                            v
                  +--------------------+
                  | Runtime Broker     |
                  | Worker Scheduler   |
                  +----------+---------+
                             |
                             v
              +--------------------------------+
              | Rust Worker Runtime            |
              | sandbox / docker / firecracker |
              +---------------+----------------+
                              |
                              v
                +------------------------------+
                | Telemetry / Logs / Artifacts |
                +------------------------------+
```

---

## 六大核心子系统

| # | 系统 | 职责 |
|---|------|------|
| 1 | Access System | API、CLI、Gateway、Auth，以及后续可扩展入口 |
| 2 | Planner System (Agent Brain) | 自然语言 → Plan → Actions |
| 3 | Task Engine | 任务生命周期、状态机、重试、并发、队列 |
| 4 | Skill System | Skill Registry、Skill Resolver、能力插件 |
| 5 | Policy Engine | 权限、Guardrail、规则拦截与执行前校验 |
| 6 | Runtime System | Rust Worker、Sandbox、隔离、Telemetry |

> 说明：表中涉及 UI、SDK、扩展策略后端等内容，部分属于设计延展，并未全部纳入当前 community core。

---

## 一、Access System

**职责**: API、CLI、Gateway、Auth，以及后续可能扩展的 UI / SDK 接入面。

### 入口

- HTTP API
- CLI (osctl)
- Gateway routes
- UI（设计延展）
- SDK（设计延展）

### 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /v1/tasks | 创建任务 |
| GET | /v1/tasks/{id} | 查询任务状态 |
| GET | /v1/tasks/{id}/stream | 任务级 SSE 遥测流 |
| GET | /v1/tasks/{id}/audit | 查询任务 audit 记录 |
| GET | /v1/tasks/{id}/replay | 查询任务 replay 记录 |
| GET | /v1/audit | 平台级 audit feed |

### CLI

```
osctl submit
osctl logs
osctl tasks
```

### 实现位置

- `internal/access/` - API 实现
- `cmd/apiserver` - HTTP 入口
- `cmd/osctl` - CLI 入口

### 备注

- 当前仓库已经提供 task / action 级 SSE 流式接口
- WebSocket 不是当前公开仓库的主线接口
- 对外路径以 `/v1/*` 为主

---

## 二、Agent Brain（Planner System）

**职责**: AgentOS 的 AI 部分，自然语言 → Plan → Actions。

### 接口

```go
type Planner interface {
    Plan(ctx context.Context, input PlanInput) (*Plan, error)
}
```

### 支持模型

- OpenAI、Claude、Gemini、DeepSeek、Local LLM

### 示例

```
User: "clone repo and run tests"

Plan:
  ├ action: git.clone
  └ action: shell.exec
```

### 实现位置

- `internal/orchestration/` - Planner 接口与适配器
- `internal/planner/` - 后续可进一步独立为 Agent Brain 包

---

## 三、Task Engine（核心调度系统）

**职责**: AgentOS 的大脑调度器。

### 功能

- 任务生命周期
- 状态机
- 重试
- 并发
- 队列

### 任务状态

```
pending → planning → queued → running → succeeded
                                    → failed
                                    → cancelled
```

### 实现位置

- `internal/orchestration/` - TaskEngine、StateMachine

---

## 四、Policy Engine（安全系统）

**职责**: 企业级 Agent 必须有 Policy，否则 AI = root 权限。

### 规则示例

```
deny rm -rf /
deny shutdown
allow docker.*
allow git.*
```

### 规则引擎选项

- OPA、Cedar、Rego

### 流程

```
Action → Policy Check → allow / deny
```

### 实现规划

- `internal/policy/` - Policy 引擎
- 详见 [Policy Engine 设计](../architecture/policy-engine.md)

---

## 五、Skill System（核心能力）

**职责**: Agent 能力插件，AgentOS 最关键的模块。

### Skill 示例

```
github.create_pr
browser.search
docker.build
shell.exec
polymarket.trade
```

### Skill 结构

```json
{
  "name": "shell.exec",
  "input": { "cmd": "string" }
}
```

### 流程

```
Action → Skill Resolver → Skill → ExecutionProfile
```

### 设计延展：AgentOS Hub（Skill Marketplace）

> 该部分属于扩展生态设想，不是当前公开仓库已承诺交付的核心范围。

- 开发者发布 skills、agents、workflows
- 抽成模式

### 实现规划

- `internal/skills/` - Skill Registry、Skill Resolver
- 详见 [Skill System 设计](../architecture/skill-system.md)

---

## 核心数据结构（DSL）

### Task

```go
type Task struct {
    ID     string
    Prompt string
    State  TaskState
    Plan   *Plan
}
```

### Plan

```go
type Plan struct {
    Actions []Action
}
```

### Action

```go
type Action struct {
    Skill string
    Input map[string]any
}
```

### ExecutionResult

```go
type Result struct {
    Status string
    Output string
}
```

Rust Worker 不理解 LLM，只执行 Action。

### 实现位置

- `pkg/taskdsl/` - Task、Plan、Action
- `api/proto/agentos/v1/` - 跨语言契约

---

## 六、Runtime Broker（Worker 调度）

**职责**: 在 Policy 与 Worker 之间，负责 Worker 调度、资源分配。

### 流程

```
Policy allow → Runtime Broker → 选择 Worker → 下发 Action
```

### 设计延展能力

> 下列能力用于说明潜在扩展方向，不代表当前开源仓库已实现或已排期。

- 多 Worker 负载均衡
- 资源预留与配额
- 优先级队列

---

## 七、事件系统

**职责**: 事件驱动架构，解耦、扩展、实时流。

### 事件命名

| 事件 | 说明 |
|------|------|
| task.created | 任务创建 |
| task.planned | Plan 生成完成 |
| action.started | Action 开始执行 |
| action.finished | Action 执行完成 |
| task.completed | 任务完成 |

### 事件流

```
API → task.created → controller → task.planned → worker → action.finished
```

### EventBus

- 默认: NATS JetStream
- Dev: memory（内存）

### 实现位置

- `internal/messaging/` - EventBus 接口
- `internal/adapters/messaging/` - memory、nats
- `pkg/events/` - 领域事件定义

---

## 八、Persistence Layer

**职责**: 系统源数据存储。

### 数据库

- PostgreSQL（默认）

### 表

| 表 | 说明 |
|------|------|
| tasks | 任务 |
| plans | 计划 |
| actions | 动作 |
| audit_logs | 审计日志 |
| skills | Skill 定义（设计延展） |
| policies | 策略（设计延展） |

### 实现位置

- `internal/persistence/` - TaskRepository 接口
- `internal/adapters/persistence/` - memory、postgres

---

## 九、Execution Runtime（Rust）

**职责**: Sandbox、命令执行、工具调用、隔离。

### Worker 架构

```
worker
 ├ grpc server
 ├ runtime manager
 ├ sandbox manager
 ├ telemetry
```

### Rust Crates

```
worker
sandbox
telemetry
```

### 隔离方式

- Docker（MVP）
- gVisor（设计探索）
- Firecracker（设计探索）

### 协议

- gRPC: `ExecuteAction`, `StreamOutput`

### 实现位置

- `runtime/crates/worker/` - gRPC RuntimeService
- `runtime/crates/sandbox/` - IsolationProvider 抽象
- `runtime/crates/telemetry/` - 流式输出模型

---

## 十、Sandbox Isolation

**职责**: 执行环境隔离。

### 支持

| Provider | 状态 |
|----------|------|
| Docker | MVP |
| gVisor | 设计探索 |
| Firecracker | 设计探索 |

### 接口

```rust
trait IsolationProvider {
    fn start(spec: SandboxSpec) -> Result<SandboxHandle>;
    fn stop(sandbox_id: &str) -> Result<()>;
}
```

---

## 十一、Telemetry & Observability

**职责**: Agent 运行可观测。

### 采集

- stdout
- stderr
- resource usage
- screenshots（设计延展）
- logs

### UI 能力（设计延展）

> 当前公开仓库已经提供 API 级 telemetry / audit / replay 能力，但并未把完整控制台 UI 作为 community core 的交付目标。

- Agent execution timeline
- step1 search repo → step2 create issue → step3 done

---

## 十二、Agent Memory（设计延展）

> 当前仓库已经具备 memory provider 与 planning / result 路径集成；本节保留的是更长周期的设计演进视角。

**职责**: 长期记忆、知识检索。

### 接口

```
MemoryProvider
```

### 实现选项

- pgvector
- qdrant
- weaviate

---

## 十三、Multi-Agent（设计延展）

> Multi-agent 编排属于后续能力层扩展，不属于当前 community core 的承诺边界。

**职责**: Agent 协作。

### 示例

```
User → ManagerAgent → WorkerAgents
  ResearchAgent
  CodingAgent
  ReviewAgent
```

---

## 项目结构

详见 [Monorepo 最终版结构](../architecture/monorepo-structure.md)。

---

## MVP 范围

**目标**: `Prompt → Plan → Action → Worker → Result`

**只实现 1 个 Skill**: `shell.exec`

**验证命令**:

```bash
osctl submit "echo hello"
# 输出: hello
```

**MVP 成功 = 上述流程端到端跑通**

---

## 产品形态（边界外延展）

> 本节用于说明开源核心之外可能出现的产品包装，不代表当前仓库范围。

| 形态 | 类比 |
|------|------|
| 开源 Runtime | Community Core |
| Hosted Agent Platform | Managed Offering |
| 企业 Agent 平台 | Private Agent Execution Platform |

---

## 商业化边界说明

> 当前公开仓库重点是开源执行底座；商业化能力仅作为边界说明，不应被理解为当前仓库交付清单。

| 形态 | 说明 |
|------|------|
| Hosted | 托管执行与控制面服务 |
| 企业增强 | 组织治理、长周期审计、权限与支持流程 |
| Ecosystem | 可能存在的技能 / agent 分发与生态层 |

---

## 相关文档

- [Monorepo 最终版结构](../architecture/monorepo-structure.md)
- [Architecture Overview](../architecture/overview.md)
- [Pluggable Adapters](../architecture/adapters.md)
- [Skill System 设计](../architecture/skill-system.md)
- [Policy Engine 设计](../architecture/policy-engine.md)
- [MVP Scope](../architecture/mvp-scope.md)
