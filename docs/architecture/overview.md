# AgentOS Architecture Overview

**AgentOS = Kubernetes for AI Agents**

Agent Infrastructure：LLM 不再直接调用工具，而是运行在 AgentOS 上。

**完整架构**: 参见 [AgentOS v1 Architecture](agentos-v1-architecture.md)。

## 6 大核心系统（概要）

| 系统 | 职责 |
|------|------|
| Access System | API、CLI、UI、SDK、Auth |
| Agent Brain | Planner、Reasoner，自然语言 → Plan |
| Task Engine | 状态机、重试、队列 |
| Skill System | Skill Registry、Skill Resolver |
| Policy Engine | 权限、Guardrail、OPA/Cedar |
| Runtime System | Rust Worker、Sandbox、Telemetry |

## 数据流

```
Clients → Access → Agent Brain → Task Engine → Skill System → Policy → Runtime Broker → Rust Worker
```

## 当前实现状态

- **Access**: HTTP、CLI 已实现
- **Agent Brain + Task Engine**: Planner、TaskEngine、SkillResolver 已实现
- **Policy Engine**: 设计完成，待实现
- **Skill System**: stub 实现，完整设计见 [skill-system.md](skill-system.md)
- **Runtime Broker**: 当前直连 Worker，未来可扩展调度
- **Messaging & Persistence**: memory、nats、postgres 适配器已实现
- **Runtime**: Rust Worker gRPC 已实现

## 设计原则

控制面依赖契约，不依赖具体实现：

- Go 依赖 `Planner`、`TaskRepository`、`EventBus`、`ExecutorClient` 等接口
- Rust 依赖 `IsolationProvider` 等 trait
- 跨语言通信仅依赖 versioned protobuf
- Messaging 和 Persistence 通过 config 可切换；默认 NATS + PostgreSQL
