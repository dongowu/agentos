# Policy Engine 设计

> Policy Engine = 企业级安全系统。在 Action 执行前做权限与 Guardrail 校验，防止 AI 乱执行。

## 定位

```
Planner → Action → Policy Engine → [deny] 终止
                        ↓ [allow]
                  SkillResolver → Execution
```

Policy 引擎是 **商业产品必须有的系统**。

---

## 核心概念

### Policy

一条策略 = 条件 + 动作。

```
IF <condition> THEN <effect>
```

- **condition**: 基于 skill、params、tenant、context 等
- **effect**: `allow` | `deny`

### 默认行为

- 无匹配策略: **allow**（白名单模式可改为 deny）
- 多条匹配: 取 **最高优先级** 或 **deny 优先**

---

## 规则示例

```
deny shell.rm
deny db.drop
allow github.*
```

语义:

- `shell.exec` 且 `params.command` 含 `rm` → deny
- `db.*` 且 action 为 `drop` → deny
- `github.*` → allow

---

## 策略 DSL（简化版）

### 结构

```go
type Policy struct {
    ID          string
    Name        string
    Priority    int      // 越大越优先
    Condition   Condition
    Effect      string   // "allow" | "deny"
}

type Condition struct {
    Skill   string   // 精确或通配 "shell.*"
    Param   string   // 如 "command"
    Op      string   // "contains", "equals", "matches"
    Value   string
}
```

### 示例

```yaml
- name: block-dangerous-shell
  priority: 100
  condition:
    skill: shell.exec
    param: command
    op: contains
    value: "rm -rf"
  effect: deny

- name: allow-github
  priority: 10
  condition:
    skill: github.*
  effect: allow
```

---

## PolicyEngine 接口

```go
type PolicyEngine interface {
    Check(ctx context.Context, action *taskdsl.Action, ctx *PolicyContext) (PolicyResult, error)
}

type PolicyContext struct {
    TaskID   string
    TenantID string
    UserID   string
}

type PolicyResult struct {
    Allowed bool
    Reason  string
}
```

**职责**:

1. 加载当前生效的策略（内存/DB）
2. 按优先级匹配 action
3. 返回 allow/deny 及原因

---

## 与 Orchestration 的衔接

```go
// TaskEngine 伪代码
func (e *Engine) executeNextAction(ctx context.Context, taskID string, action *taskdsl.Action) error {
    if e.policy != nil {
        result, err := e.policy.Check(ctx, action, policyContext)
        if err != nil {
            return err
        }
        if !result.Allowed {
            return e.failTask(ctx, taskID, result.Reason)
        }
    }
    // 继续 SkillResolver → ExecutorClient
    ...
}
```

---

## 策略存储

### Phase 1（MVP 后）

- 内存或 YAML 文件
- 启动时加载，无热更新

### Phase 2

- `policies` 表
- 支持 API 增删改
- 支持租户隔离、版本

### Phase 3

- 策略组合、继承
- 审计日志：每次 Check 记录

---

## 实现规划

### Phase 1

- `internal/policy/` 包
- `PolicyEngine` 接口
- 内存实现：从 YAML 加载策略
- TaskEngine 集成：可选注入，dev 模式可跳过

### Phase 2

- PostgreSQL `policies` 表
- PolicyRepository
- 管理 API

### Phase 3

- 复杂条件（AND/OR）
- 租户策略继承
- 策略测试/模拟

---

## 文件结构（Phase 1）

```
internal/
  policy/
    engine.go      # PolicyEngine 接口
    checker.go     # 内存实现
    model.go       # Policy, Condition 等
    loader.go      # YAML 加载
```

---

## 安全考虑

1. **deny 优先**: 有歧义时默认 deny
2. **审计**: 所有 Check 结果写 audit_log
3. **不可绕过**: Policy 必须在 Go 控制面执行，Worker 不信任客户端
4. **可观测**: 策略命中情况可打点、告警
