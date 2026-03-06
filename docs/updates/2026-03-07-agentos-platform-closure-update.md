# AgentOS 平台闭环修复更新

- 日期：2026-03-07
- 受众：AgentOS 平台 / 基础设施 / 应用研发团队
- 目的：同步本轮“查漏补缺”修复范围、验证结果、当前剩余缺口与下一步建议

## TL;DR

本轮修复已经把 AgentOS 从“基础设施组件大多存在、但主链路没有完全闭环”的状态，推进到“controller / apiserver / worker 可以共享注册信息并完成调度；agent 上下文能够进入 policy / memory / vault；默认 planner 不再退化为固定 `echo ok`”的状态。

结论上，这一批优先级问题里：

- `P0` 共享注册与调度闭环：已完成
- `P1` 真实 `AgentName` 进入 policy：已完成
- `P1` `MemoryHook` 接入 planning / result store：已完成
- `P1` Vault 配置装载与 token 注入：已完成
- `P2` 默认 planner 行为提升：已完成
- Rust sandbox 侧遗留红测与 deprecation warning：已顺手完成

## 本轮完成了什么

### 1. 打通 controller / apiserver / worker 的共享注册闭环

控制面新增了 `ListWorkers` gRPC 能力，controller 现在不仅能接收 worker 注册、心跳、注销，还能把当前 worker 快照对外暴露给远端调度方。apiserver/调度侧新增了 remote registry 客户端，可以通过 controller 查询当前 worker 列表与可用 worker，而不是再使用本地孤立的内存注册表。

同时，worker pool 现在有了真实的 gRPC dialer，会按 worker 地址建立 runtime 连接并缓存；这意味着调度不再停留在“能选 worker 但没有共享注册源”的半闭环状态，而是变成了真正可工作的控制面查询 + 运行面直连模型。

### 2. 让 agent 上下文真正进入主执行链路

`/agent/run` 入口现在会把 `agent_name` 透传到 task submission API，再由 orchestration 层的 `StartTaskWithInput(...)` 统一接收 `Prompt / AgentName / TenantID`。这让原来“HTTP 层知道 agent，但执行层不知道 agent”的断层被补上。

修复后，policy 评估使用的已经是请求中的真实 `AgentName` 和 `TenantID`，不再是空值或默认值。这意味着后续做按 agent 授权、按 tenant 限流、按角色策略扩展时，数据面已经具备正确的上下文输入。

### 3. 接上 MemoryHook 与 Vault

内存能力这次不再只是“provider 已实现但悬空”：

- planning 前会先做 recall，把相关历史结果拼入 planning prompt
- action 执行完成后，无论 direct execute 还是 scheduler 结果回收，都会把结果写入 memory

凭据能力这次也完成了最小闭环：

- 支持通过配置 / 环境变量加载 agent secret
- engine 在 action 下发前会从 vault 获取 opaque token
- token 会以 `AGENTOS_CREDENTIAL_TOKEN` 的形式注入到 action payload 的环境变量中

当前实现遵循“worker 只看到 token，不直接看到真实 secret”的方向，为后续控制面 secret resolve 打下基础。

### 4. 默认 planner 从 stub 提升为 prompt-preserving fallback

此前最影响体验的问题之一，是未配置 LLM 时 planner 直接返回固定 `echo ok`，导致用户 prompt 基本被忽略。这次默认回退行为已调整为：保留原始 prompt，生成单步 fallback command。

这意味着在没有接入完整 LLM planner 的情况下，系统至少会尝试执行用户的原始意图，而不是始终跑一个固定占位命令。虽然它还不等于真正的多步智能规划，但已经从“完全不可用的 stub”升级为“可工作的保底策略”。

### 5. 顺手收掉 Rust sandbox 遗留问题

在做全量验证时，Rust workspace 里有一条 `docker_runtime` 相关测试会因为环境中 `docker` 命令存在但执行卡住，而报出纯 `timeout` 文案，导致测试期望不匹配。该问题已修复：

- Docker timeout 现在会带上 `docker` 上下文
- Native timeout 仍保持原有通用语义
- 相关回归测试已补齐
- 原来使用 deprecated legacy API 的 sandbox 测试文件已清理，workspace 测试输出也更干净

## 验证结果

本轮实际完成了以下验证：

### Go

已执行：

```bash
go test ./...
```

结果：通过。

### Rust

已执行：

```bash
cargo test --workspace
```

结果：通过。

## 当前剩余缺口

这批问题已经把“执行链路闭环”补上，但还存在下一阶段值得优先投入的点：

### 1. Planner 仍然只有保底能力，没有真正的高质量任务拆解

虽然默认 planner 已不再是 `echo ok`，但在没有 LLM provider 时，它仍然只是“把 prompt 原样包成单步 command”。这解决了可用性问题，但还没有解决复杂任务拆解、工具选择、多步计划生成等“智能层”问题。

### 2. Vault 已完成 token 注入，但更完整的 secret 使用链路还可以继续增强

当前已经完成“加载 secret -> 生成 opaque token -> 注入 action env”，但如果后续要支持更复杂的外部 agent / provider / runtime secret 使用场景，仍建议继续补完 token resolve 与生命周期管理策略。

### 3. Auth / Tenant / 多租户隔离还未完成

虽然 `TenantID` 已能穿透到 orchestration / policy，但真正的认证、租户隔离、租户级配额、租户级审计仍属于下一阶段能力，不在本轮范围内。

### 4. 还缺一次更贴近真实运行方式的端到端验收

当前单元/集成测试已经通过，但如果要把“平台闭环已通”对外讲得更稳，建议补一套基于真实 `controller + apiserver + worker` 进程组合的 acceptance 流程，验证注册、调度、执行、结果回写、health monitor 等关键路径。

## 建议下一步

建议按下面顺序继续推进：

1. 补真实多进程 E2E / acceptance 流程，固化“共享注册与调度闭环”的验收证据
2. 引入更像样的 planner 默认策略，或默认启用 LLM planner
3. 推进 auth / tenant / quota / audit，补齐多租户平台能力
4. 继续增强 vault 的 token 生命周期、轮换与外部 provider 对接能力

## 一句话版本

本轮修复已经把 AgentOS 的控制面注册、调度、agent 上下文、memory、vault、默认 planner 和 Rust sandbox 验证链路全部串起来，平台从“组件齐但没闭环”进入“核心主链路已闭环、可继续向智能层和多租户层迭代”的阶段。
