# 核心能力参考

这份文档承接了首页 README 中不再内联展开的能力细节。

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

## 内置工具

| 工具 | 说明 |
|------|------|
| `shell` | 通过具备沙箱能力的运行时执行 Shell 命令 |
| `file.read` | 读取文件内容 |
| `file.write` | 写入文件并自动创建缺失目录 |
| `git.clone` | 克隆 Git 仓库 |
| `git.status` | 查看 Git 工作树状态 |
| `http.get` | 发起 HTTP GET 请求 |
| `http.post` | 发起 HTTP POST 请求 |

## 可插拔适配器

| 接口 | 适配器 | 默认值 |
|------|--------|--------|
| `EventBus` | memory、nats | 生产：`nats`；开发：`memory` |
| `TaskRepository` | memory、postgres | 生产：`postgres`；开发：`memory` |
| `AuditLogStore` | memory、postgres | 生产：`postgres`；开发：`memory` |
| `Planner` | prompt、注册表驱动的 LLM provider（内置 `openai`） | prompt 基线；带有限重试、repair 后再 fallback |
| `Memory.Provider` | inmemory、redis | `inmemory` |
| `RuntimeAdapter`（Rust） | native、docker | `native` |
| `Scheduler` | local、nats | 生产：`nats`；开发：`local` |

## 安全模型

### Go 控制面

- `PolicyEngine` 以 deny 优先的方式执行 allow / deny 规则
- `PolicyEngine` 可以把配置命中的工具模式以 `approval required` 的治理门禁原因阻断
- 自主级别覆盖 `supervised`、`semi`、`autonomous`
- `CredentialVault` 使用不透明 agent token，避免通用任务载荷直接携带真实密钥
- 单 agent 限流控制 action 频率
- 危险命令检查在执行前拦截高风险模式

### Rust worker 运行时

- `SecurityPolicy` 在执行前校验命令与路径
- 环境隔离会清空进程环境变量，只回注安全变量
- secret redaction 会对 API key、Bearer token 等输出进行脱敏
- 输出截断限制结果大小
- 单 action 超时保护 worker 健康
- Docker runtime 可强制 `--read-only`、`--network none` 与资源限制

## 分布式架构

高层上看：

- `controller` 负责共享注册中心与 worker 协调路径
- `apiserver` 接收 HTTP 任务、audit 请求、replay 请求与 SSE 订阅
- orchestration core 负责 planning、policy 检查、dispatch 与结果处理
- worker 选择通过共享 registry / pool 路径完成
- 分布式模式可使用 NATS 调度；开发模式仍可保留本地直连执行

如需端到端验证，请运行 [`./scripts/acceptance.sh`](../../scripts/acceptance.sh)，并阅读[多进程验收说明](../architecture/multiprocess-acceptance.md)。

## 仓库结构

- `api/` — protobuf 契约与生成后的 gRPC 绑定
- `cmd/` — `apiserver`、`controller`、`claw-cli`、`osctl` 等入口
- `internal/access/` — HTTP handler、CLI wiring、鉴权逻辑
- `internal/adapters/` — LLM、memory、messaging、persistence、runtime client
- `internal/agent/` — agent DSL、运行时元数据与 manager wiring
- `internal/bootstrap/` — 配置驱动的依赖装配
- `internal/orchestration/` — task engine、planner、状态机逻辑
- `internal/policy/` — policy engine 与 credential vault
- `internal/scheduler/` — 本地与 NATS 调度路径
- `internal/tool/` — 内置工具实现与 tool bridge 接口
- `internal/worker/` — registry、pool、health monitor
- `pkg/` — 共享 config、events、task DSL 类型
- `runtime/` — Rust worker、sandbox adapter、telemetry crate
- `deploy/` — NATS、Postgres 等本地基础设施清单
- `examples/` — 示例 agents 与 tasks

## Open Core 边界

AgentOS 以 `Apache-2.0` 发布平台核心，并把商业化包装放在仓库边界之外。

- **Community** — 可自部署的控制面、worker runtime、调度、audit API、replay、telemetry 与 agent-loop 底座
- **Enterprise（未来）** — 组织治理、SSO / SCIM / RBAC、长周期审计中心、支持流程
- **Cloud（未来）** — 托管控制面、运维控制台、升级、计费与 SLA 能力

当前边界定义见[许可证决策](../strategy/licensing-decision.md)与[平台层与能力层边界](../architecture/platform-vs-capability-boundary.md)。
