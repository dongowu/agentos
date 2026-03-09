# ClawOS v1 架构（一步到位）

> **状态说明**：这是一份历史性的概念 / 对照文档，用来表达一个更偏产品化的目标态设想；它**不是**当前 AgentOS 开源仓库的正式 roadmap，也不是公开仓库已经实现的能力清单。
>
> **阅读建议**：如果你想了解当前仓库的真实能力，请优先阅读 [Architecture Overview](overview.md)、[Getting Started](../guides/getting-started.md) 与 [Core Capabilities Reference](../reference/core-capabilities.md)。
>
> **概念定位**：ClawOS = Agent Runtime + Tool Marketplace + Execution Sandbox 的产品化设想。

## 能力覆盖

```
Agent 运行
Tool 插件
Browser 自动化
Shell 自动化
Workflow
Memory
多 Agent 协作
```

## 对标

| 能力 | OpenClaw | ZeroClaw | HiClaw | ClawOS |
|------|----------|----------|--------|--------|
| Agent Runtime | ❌ | ❌ | ❌ | ✅ |
| Tool Marketplace | ❌ | ❌ | ❌ | ✅ |
| Workflow | ❌ | ❌ | ❌ | ✅ |
| Memory | ❌ | ❌ | ❌ | ✅ |
| Multi Agent | ❌ | ❌ | ❌ | ✅ |
| Sandbox | ⚠️ | ⚠️ | ⚠️ | ✅ |

**本质区别（概念层）**: 这里讨论的是一个更完整的 agent product shell 设想，而不是当前 AgentOS 仓库已经公开交付的开源执行底座。

---

## 总体架构

> 说明：以下架构图是概念目标态，包含 UI、Marketplace、WASM 等不属于当前 AgentOS community core 的内容。

```
                ┌──────────────────────┐
                │     Claw Studio       │
                │  Web UI / Agent IDE   │
                └───────────┬───────────┘
                            │
                     ┌──────▼──────┐
                     │ API Gateway  │
                     └──────┬──────┘
                            │
              ┌─────────────▼─────────────┐
              │      Agent Runtime        │
              │  Planner  Memory  Queue   │
              │     │        │       │     │
              │     └────┬───┴───────┘     │
              │         ▼                 │
              │    Task Engine           │
              └───────┬──────────┬───────┘
                      │          │
              ┌───────▼───┐ ┌────▼────────┐
              │ Tool Exec │ │ Agent Worker│
              └───────┬───┘ └────┬────────┘
                      │         │
       ┌──────────────▼────┐ ┌──▼─────────────┐
       │   Tool Plugins    │ │ Agent Plugins  │
       └──────────────┬────┘ └────┬──────────┘
                      │           │
              ┌───────▼───────────▼──────┐
              │    Execution Sandbox     │
              │ Browser/Shell/Docker/WASM │
              └──────────────────────────┘
```

---

## Monorepo 结构（一步到位）

> 说明：以下目录结构是概念化拆分方案，并非当前仓库目录现状。

```
clawos/
│
├── cmd/
│   ├── clawd          # 服务端 daemon
│   └── claw-cli       # CLI
│
├── runtime/
│   ├── engine         # Task Engine
│   ├── planner        # LLM Planner
│   ├── scheduler      # 任务调度
│   └── workflow       # Workflow 引擎
│
├── agent/
│   ├── loader         # agent.yaml 加载
│   ├── executor       # Agent 执行器
│   └── registry       # Agent 注册
│
├── tool/
│   ├── registry       # Tool 注册表
│   ├── executor       # Tool 执行器
│   └── sdk            # Tool 开发 SDK
│
├── sandbox/
│   ├── browser        # Playwright
│   ├── shell          # Shell 执行
│   ├── docker         # Docker 容器
│   └── wasm           # WASM 沙箱
│
├── memory/
│   ├── short          # Redis 短期
│   ├── vector         # Vector DB 长期
│   └── storage        # 存储抽象
│
├── gateway/
│   ├── http           # REST API
│   ├── websocket      # 流式
│   └── mcp            # MCP 协议
│
├── sdk/
│   ├── go
│   └── python
│
├── studio/
│   └── web-ui         # Claw Studio 前端
│
└── examples/
```

---

## 技术栈

| 层级 | 选型 |
|------|------|
| 核心 | Go |
| 执行层 | Rust（可选，sandbox 可先用 Go+Playwright） |
| 存储 | Postgres、Redis、Vector DB |
| 任务队列 | NATS / Kafka |
| Browser | Playwright |
| 容器 | Docker、Firecracker |
| LLM | OpenAI、Claude、DeepSeek、Local LLM |

---

## 核心接口

### Tool 接口

```go
package tool

type Tool interface {
    Name() string
    Description() string
    Run(ctx context.Context, input map[string]any) (any, error)
}

func Register(t Tool)
func Run(name string, args map[string]any) (any, error)
```

### Agent 配置（agent.yaml）

```yaml
name: trading-agent

model: deepseek-chat

memory:
  type: redis

tools:
  - browser
  - shell
  - polymarket

workflow:
  - plan
  - execute
  - reflect
```

Agent 是配置，不是代码。

### Memory 接口

```go
package memory

type Provider interface {
    Put(ctx context.Context, key string, value []byte) error
    Get(ctx context.Context, key string) ([]byte, error)
    Search(ctx context.Context, query string, k int) ([]SearchResult, error)
}
```

- 短期: Redis
- 长期: Vector DB

---

## 执行流程

```
User Task
   │
   ▼
Gateway
   │
   ▼
Runtime
   │
   ▼
Planner
   │
   ▼
Task List
   │
   ▼
Tool Executor
   │
   ▼
Sandbox
   │
   ▼
Result
```

---

## Sandbox 类型

> 说明：其中 `wasm` 等项属于设想能力，不代表当前仓库已有对应 runtime 实现。

| 类型 | 说明 |
|------|------|
| browser | Playwright，open/click/input/scrape |
| shell | 命令执行 |
| docker | 容器内执行 |
| python | Python 脚本 |
| wasm | WASM 沙箱 |

---

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /agent/run | 运行 Agent |
| GET | /agent/status | 查询状态 |
| POST | /tool/run | 直接调用 Tool |
| GET | /workflow | 查询 Workflow |

### 示例

```bash
POST /agent/run
```

```json
{
  "agent": "trading-agent",
  "task": "buy polymarket position"
}
```

---

## CLI

```
claw run agent.yaml
claw dev
claw tool install
claw agent publish
```

---

## Claw Studio（Web UI）

> 该部分是产品层设想，不属于当前公开仓库交付范围。

- Agent Builder
- Workflow Builder
- Logs / Debug
- Tool Marketplace

类似 Langflow。

---

## 多 Agent 协作

```
research-agent → strategy-agent → execution-agent
```

流程: research → strategy → execution

---

## 第一版必须实现

```
runtime
tool
agent
browser sandbox
shell sandbox
gateway
cli
```

即可发布。

---

## 概念化商业模式

> 本节是概念讨论，不代表当前开源项目的正式商业承诺。

**Agent Marketplace**：作为概念示例，别人卖 agent，平台可抽成。

---

## 与现有 AgentOS 的映射

| AgentOS | ClawOS |
|---------|--------|
| cmd/apiserver | cmd/clawd + gateway/http |
| cmd/osctl | cmd/claw-cli |
| internal/orchestration | runtime/engine, runtime/planner |
| internal/adapters/* | tool/, sandbox/, memory/ |
| pkg/taskdsl | runtime/engine 内 |
| runtime/crates/worker | sandbox/* |

---

## 迁移路径

1. **保留** Go 控制面、NATS、Postgres
2. **新增** tool/、agent/、sandbox/browser、memory/
3. **重命名** cmd → clawd、claw-cli
4. **新增** gateway/、studio/
5. **逐步** 替换 orchestration 为 runtime/

---

## 已实现（可直接开干）

| 包 | 文件 | 说明 |
|------|------|------|
| internal/tool | interface.go, registry.go, executor.go | Tool 接口、注册、执行 |
| internal/tool/builtin | shell.go | shell 内置 Tool |
| internal/agent | config.go, loader.go | agent.yaml 加载 |
| internal/memory | provider.go | Memory 接口 |
| internal/sandbox | interface.go | Sandbox 接口 |
| examples/agent | trading-agent.yaml | 示例 Agent 配置 |

### 使用示例

```go
import (
    _ "github.com/dongowu/agentos/internal/tool/builtin"
    "github.com/dongowu/agentos/internal/tool"
)

// 执行 shell
out, err := tool.Run(ctx, "shell", map[string]any{"cmd": "echo hello"})

// 加载 agent
cfg, err := agent.Load("examples/agent/trading-agent.yaml")
```
