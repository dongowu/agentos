# AgentOS v1 Architecture

> **定位**: AgentOS = Kubernetes for AI Agents  
> **一句话**: AgentOS 是 Agent 基础设施（Agent Infrastructure）。

```
LLM 不再直接调用工具
而是运行在 AgentOS 上
```

## 对标

| 系统 | 职责 |
|------|------|
| Kubernetes | container orchestration |
| Ray | distributed compute |
| LangChain | agent SDK / LLM workflow |
| **AgentOS** | **Agent Infrastructure** |

## AgentOS vs 市面框架

| 框架 | 类型 |
|------|------|
| LangChain | LLM workflow |
| AutoGPT | agent demo |
| CrewAI | agent orchestration |
| OpenClaw | computer use |
| HiClaw | CLI agent |
| **AgentOS** | **Agent Infrastructure** |

---

## 总体架构（最终形态）

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
| 1 | Access System | API、CLI、UI、SDK、Auth |
| 2 | Planner System (Agent Brain) | 自然语言 → Plan → Actions |
| 3 | Task Engine | 任务生命周期、状态机、重试、并发、队列 |
| 4 | Skill System | Skill Registry、Skill Resolver、能力插件 |
| 5 | Policy Engine | 权限、Guardrail、OPA/Cedar/Rego |
| 6 | Runtime System | Rust Worker、Sandbox、隔离、Telemetry |

---

## 一、Access System

**职责**: API、CLI、UI、SDK、Auth。

### 入口

- HTTP API
- WebSocket
- CLI (osctl)
- UI（未来）
- SDK（Go、TypeScript、Python）

### 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /tasks | 创建任务 |
| GET | /tasks/{id} | 查询任务状态 |
| GET | /tasks/{id}/stream | 流式输出 |

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
- `internal/planner/` - 未来可独立为 Agent Brain 包

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
- 详见 [Policy Engine 设计](policy-engine.md)

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

### 未来：AgentOS Hub（Skill Marketplace）

- 开发者发布 skills、agents、workflows
- 抽成模式

### 实现规划

- `internal/skills/` - Skill Registry、Skill Resolver
- 详见 [Skill System 设计](skill-system.md)

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

### 未来能力

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
| skills | Skill 定义（未来） |
| policies | 策略（未来） |

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
- gVisor（未来）
- Firecracker（未来）

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
| gVisor | 未来 |
| Firecracker | 未来 |

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
- screenshots（未来）
- logs

### UI 能力（未来）

- Agent execution timeline
- step1 search repo → step2 create issue → step3 done

---

## 十二、Agent Memory（可选，未来）

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

## 十三、Multi-Agent（未来）

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

详见 [Monorepo 最终版结构](monorepo-structure.md)。

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

## 产品形态（未来）

| 形态 | 类比 |
|------|------|
| 开源 Runtime | Kubernetes |
| SaaS Agent Platform | Zapier + AI |
| 企业 Agent 平台 | Private Agent Infrastructure |

---

## 商业化方向

| 形态 | 说明 |
|------|------|
| SaaS | Agent Cloud，按 execution minutes / workers / storage 收费 |
| 企业版 | on-prem agent platform，客户：银行、云厂、AI 公司 |
| Skill Marketplace | AgentOS Hub，抽成 20% |

---

## 相关文档

- [Monorepo 最终版结构](monorepo-structure.md)
- [Architecture Overview](overview.md)
- [Pluggable Adapters](adapters.md)
- [Skill System 设计](skill-system.md)
- [Policy Engine 设计](policy-engine.md)
- [MVP Scope](mvp-scope.md)
