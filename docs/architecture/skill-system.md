# Skill System 设计

> Skill System = 工具市场。将 Action 映射到可执行配置，是 AgentOS 与普通 Agent 项目拉开差距的核心之一。

## 定位

```
Action → SkillResolver → Skill → Runtime
```

Rust Worker 只执行「已解析的 Skill」，不关心 LLM 或 Plan 语义。

---

## 核心概念

### Skill

```go
type Skill struct {
    Name             string            // 如 "shell.exec"
    Description      string            // 人类可读描述
    Parameters       *ParameterSchema  // 参数定义
    ExecutionProfile string            // 对应 Runtime 的 profile，如 "sandbox"
}
```

### ParameterSchema

```go
type ParameterSchema struct {
    Type       string            // "object", "string", "array" 等
    Properties map[string]Prop   // JSON Schema 风格
    Required   []string
}

type Prop struct {
    Type        string
    Description string
    Default     interface{}
}
```

### Action（来自 Plan DSL）

```go
type Action struct {
    Type   string                 // 对应 Skill.Name
    Params map[string]interface{} // 对应 Skill.Parameters
}
```

---

## SkillResolver 接口

```go
type SkillResolver interface {
    Resolve(action *taskdsl.Action) (runtimeProfile string, payload []byte, err error)
}
```

**职责**:

1. 根据 `action.Type` 查找 Skill
2. 校验 `action.Params` 符合 Skill.Parameters
3. 返回 `runtimeProfile`（Worker 用哪个隔离环境）和 `payload`（序列化后的执行参数）

---

## Skill 注册表

### 内置 Skill（MVP）

| Skill | 说明 | ExecutionProfile |
|-------|------|-------------------|
| shell.exec | 执行 shell 命令 | sandbox |

### 扩展方式

1. **静态注册**: 代码内 `RegisterSkill(shell.ExecSkill)`
2. **配置驱动**: YAML/JSON 定义，启动时加载
3. **数据库**: `skills` 表，支持运行时增删（未来）

### 注册接口

```go
type SkillRegistry interface {
    Register(skill *Skill) error
    Get(name string) (*Skill, error)
    List() []*Skill
}
```

---

## 执行流程

```
1. Planner 产出 Plan，内含 Action{Type: "shell.exec", Params: {command: "echo hello"}}
2. TaskEngine 取下一个 Action，调用 SkillResolver.Resolve(action)
3. SkillResolver 查 Skill "shell.exec"，校验 Params，返回 profile="sandbox", payload=...
4. TaskEngine 通过 ExecutorClient 调用 Worker: ExecuteAction(task_id, action_id, "sandbox", payload)
5. Worker 按 profile 选 IsolationProvider，执行 payload 中的命令
```

---

## 与 Policy Engine 的衔接

```
Action → PolicyEngine.Check(action) → [deny] 终止
                    ↓ [allow]
              SkillResolver.Resolve(action)
```

Policy 在 Resolve 之前执行，可基于 `action.Type` 和 `action.Params` 做规则匹配。

---

## 数据流

```
Plan.Actions[i]
    ↓
PolicyEngine.Check()  // 可选，未来
    ↓
SkillResolver.Resolve()
    ↓
(runtimeProfile, payload)
    ↓
ExecutorClient.ExecuteAction(..., profile, payload)
```

---

## 实现规划

### 当前状态（2026-03-08）

- `internal/skills/` 已落地，提供内存注册表与 `Resolver`
- 内置 action kinds：`command.exec`、`file.read`、`file.write`、`http.request`、`browser.step`
- `SkillResolver` 现在会做 action kind 查找、基础 payload 校验，并解析 runtime profile
- `EngineImpl` 不再静默吞掉 resolver 错误，解析失败会直接让任务进入 `failed`
- `file.read`、`file.write`、直接 tool kind 与 `http.request` 已可通过 control-plane tool bridge 执行
- 当前执行面仍然是“bridge + command runtime”混合模式；浏览器类 action 与更专用的独立执行器仍未完整落地

### 下一阶段

- 支持从 YAML 加载 Skill 定义
- 扩展更细的参数 schema 与 tenant 级 skill catalog

### Phase 3

- `skills` 表，PostgreSQL
- 支持通过 API 增删 Skill
- 支持 Skill 版本、租户隔离

---

## 当前文件结构

```
internal/
  skills/
    registry.go      # Skill 注册表与定义
    resolver.go      # SkillResolver 实现，负责校验与 profile 解析
    builtin.go       # 当前内置 action kind 定义
```

---

## 示例：shell.exec Skill 定义

```yaml
name: shell.exec
description: Execute a shell command in sandbox
parameters:
  type: object
  properties:
    command:
      type: string
      description: The command to run
  required:
    - command
execution_profile: sandbox
```
