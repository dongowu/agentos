# ClawOS v1 架构（一步到位）

> **产品**: ClawOS  
> **定位**: Agent Runtime + Tool Marketplace + Execution Sandbox  
> **一句话**: The Operating System for AI Agents

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

**本质区别**: 他们是工具，ClawOS 是操作系统。

---

## 总体架构

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

## 商业模式

**Agent Marketplace**：别人卖 agent，ClawOS 抽成。

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
