# 产品方向：Governed Run

- 状态：作为 `v0.2` 的工作产品方向
- 日期：2026-11-16
- 读者：产品、平台、运行时与开发者体验维护者

## 决策

AgentOS 是 **面向内部 coding / ops Agent 的自托管可治理执行平面**。

它不是通用 Agent 框架、聊天产品、工作流市场或企业控制台。

首批理想客户是需要在自有基础设施上运行内部 coding / ops 自动化的平台、基础设施或工程效率团队。

产品承诺是：

> 每一次 Agent 运行都产出一份可审查的执行证据：谁运行、执行了哪个工具或动作、策略如何判定、在哪里执行、返回了什么、以及如何回放。

## 为什么选择这个切入点

仓库已经具备难以快速补齐的执行资产：Go 控制面、Rust worker、local/NATS 调度、worker 注册与容量、policy、audit、replay、SSE，以及真实多进程验收。

真正缺的不是再做一个 planner 或再增加一批工具，而是让默认 Agent 执行路径获得统一、强制的治理边界。

框架负责描述 Agent；AgentOS 应负责让 Agent 可安全运行、可解释、可追责。这个定位更窄，也更难被普通 Agent 框架替代。

## 黄金路径

首次评估应在一个问题内完成：

> 我能否运行一个 Agent 任务，并准确看到发生了什么？

```bash
# 终端 1：启动执行 worker
cd runtime && cargo run -p agentos-worker

# 终端 2：运行任务并输出 replay
AGENTOS_MODE=dev AGENTOS_WORKER_ADDR=localhost:50051 \
  go run ./cmd/osctl run "echo hello"

# 启动多进程栈后，需要从已连接 API 获取机器可读证据时
go run ./cmd/osctl --server http://127.0.0.1:18080 --token dev-token replay <task-id> --json
```

`osctl run` 负责提交任务、等待终态并展示 replay。成功输出至少应包含任务状态、动作数、命令、退出码、可用的 worker/runtime 信息，以及捕获到的输出。在独立嵌入式模式下，replay 在同一次调用中可见；稍后通过 `osctl replay` 查询时，需要连接保留任务状态的 API。

## 产品契约

以下条件决定一次运行是否真正“可治理”：

1. 每个对外可见的 tool call 在执行前都经过 policy 判定。
2. 被拒绝的调用不得执行，并留下解释拒绝原因的 audit 记录。
3. 每个已执行调用都记录动作、结果和相关运行上下文。
4. 已完成任务可以通过 replay 投影被检查。
5. 默认 CLI/API 路径无需直接访问数据库即可获得这些证据。

本地开发捷径可以存在，但必须显式说明；不能把未经治理的路径描述为 governed execution。

## 发布范围

### 现在：让治理可见

- 将 `osctl run` 与 `osctl replay` 作为首体验
- LLM tool-calling 路径同样执行 policy 判定并写 audit
- planned action 继续通过 `agentos-worker` 执行
- 示例只引用真实存在的工具
- acceptance 覆盖允许执行、拒绝执行与 replay

### 下一步：让 Agent profile 成为真契约

- 未注册工具直接失败，不再静默跳过
- `tools`、`policy`、`model`、`memory` 声明真实影响运行
- 提供一个仅使用 file、Git、shell、HTTP 内建工具的 reference coding/ops workflow
- 需要沙箱与执行证据的进程型 tool call 统一经过 worker 边界

### 后续：证明重复使用

- 发布文档化的 `v0.2` governed-run 契约
- 与少量内部工程团队完成自托管试点
- 只发布可测量结果，不扩大平台叙事

## 成功指标

以下指标是发布门槛，不是当前实测值：

| 维度 | 目标 |
|------|------|
| 激活 | 新开发者在 10 分钟内完成一次 governed run 并打开 replay |
| 治理 | 100% tool call 有 policy 判定与 audit 记录 |
| 拒绝安全 | 100% 被拒绝调用没有产生可观察副作用 |
| 可回放 | 至少 95% 的试点完成任务可从持久化证据回放 |
| 工作负载质量 | 文档化 reference coding/ops workload 成功率至少 80% |
| 采用 | 至少两个试点团队每周重复运行 governed task |

若指标未达成，优先修复治理主路径，不增加工具、控制台或垂直包。

## 非目标

- 通用聊天产品或个人助理
- prompt IDE、低代码工作流编辑器或 Agent 市场
- 开源核心中的浏览器自动化、Web3 交易或行业垂直包
- 本轮生产级 dashboard 或企业控制台
- 开源核心中的企业 SSO、SCIM、RBAC、组织治理或长期审计服务
- 以多 Agent 编排作为头号能力

## 产品边界

### 开源核心

执行编排、runtime、policy、audit、replay、worker registry、调度、telemetry、CLI 与参考开发者体验。

### 商业未来

组织治理、身份集成、托管控制、长期审计运营、托管基础设施与支持。企业能力应通过接口扩展平台，而不是分叉执行与治理模型。

## 暂缓决策

以下决策暂不阻塞 governed-run 方向：

- 首个外部身份提供方
- 托管云打包模式
- 非 coding 垂直包是否成为独立产品线
- `claw` 与 `osctl` 的长期关系

在这些决策明确前，`osctl` 是主操作入口，ClawOS 材料保持概念或兼容性质。

## 本方向的验收证据

- `osctl run` 到达终态并展示 replay 证据
- `osctl replay --json` 返回完整 replay 契约
- 被拒绝的 tool call 在执行前被阻断并写入 audit
- 被允许的 LLM tool call 经过 policy 并写入 audit
- Go 测试与多进程 acceptance 保持通过

