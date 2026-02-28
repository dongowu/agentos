# AI Orchestrator (Rust)

一个面向多 Agent 协作的执行编排器：输入需求，输出可追踪、可回滚、可审批的交付结果。

## 核心能力

- 多阶段工作流编排（`mvp` / `default` / `autonomy`）
- 异步作业模型（`submit` -> `work` -> `result`）
- 人工闸门（Human Gate）审批 + `resume` 续跑
- 讨论会话收敛机制（converge / expire / escalate）
- Git checkpoint / rollback 安全回退
- Shell / 文件系统硬规则防护（Execution Guard）
- 轻量 RAG 上下文检索 + 产物沉淀
- 全链路追踪（job + decision + conversation + message）

## 目录结构

```text
.
├── Cargo.toml
├── config/
│   └── workflows.yaml
├── data/                  # sqlite 与运行数据
├── src/
│   ├── main.rs
│   ├── cli.rs
│   ├── jobs.rs
│   ├── pipeline.rs
│   ├── workflow_config.rs
│   ├── messaging.rs
│   ├── rag.rs
│   ├── guard.rs
│   └── gitops.rs
└── ROADMAP.md
```

## 快速开始

### 1) 环境要求

- Rust stable（建议 `1.76+`）
- Git

### 2) 安装与验证

```bash
cargo build
cargo test
```

### 3) 一条命令跑通

```bash
cargo run -- run "实现一个带登录能力的 MVP" --workflow default
```

## CLI 常用命令

```bash
# 提交异步任务
cargo run -- submit "实现支付重试机制" --workflow autonomy

# 消费队列
cargo run -- work --limit 8

# 查询状态/结果
cargo run -- job-status <job_id>
cargo run -- result <job_id>

# 查看待审批项
cargo run -- pending

# 审批并续跑
cargo run -- approve <decision_id>
cargo run -- resume <job_id>

# 查看完整追踪
cargo run -- trace <pipeline_id>

# 企业多团队模式（插件化角色编排）
cargo run -- team-run "交付一个用户登录系统" --max-parallel 3

# 运行时动态调整策略（命令行覆盖）
cargo run -- team-run "交付一个用户登录系统" --gate-policy majority --arbiter-policy immediate_escalation

# 切换跨团队合并策略
cargo run -- team-run "交付一个用户登录系统" --team-topology multi --merge-policy fast

# 合并冲突自动回滚+重试编排
cargo run -- team-run "交付一个用户登录系统 [[merge:conflict]]" --team-topology multi --merge-policy strict --enable-merge-auto-rework --max-merge-retries 2

# 按冲突类型路由自动修复子流程
cargo run -- team-run "交付一个用户登录系统 [[merge:api-conflict]]" --team-topology multi --merge-policy strict --enable-merge-auto-rework

# 启用多团队拓扑（跨团队并行收敛）
cargo run -- team-run "交付一个用户登录系统" --team-topology multi --max-parallel 4 --max-parallel-teams 2

# 启用动态角色故障切换（同一角色多实例）
cargo run -- team-run "交付一个用户登录系统 [[failover:coder]]" --enable-role-failover --max-role-attempts 2

# 通过配置文件加载策略
cargo run -- team-run "交付一个用户登录系统" --profile-file config/team-runtime.yaml
```

`team-run` 使用进程内 Rust 插件注册表（trait + registry）运行企业流程：
- 5 部门 Board：Product / Engineering / QA / Security / Ops
- 4 个 Gate：Intake / Freeze / Release / Closure
- Gate 规则：`unanimous` / `majority`
- 冲突处理：`two_round` / `immediate_escalation`
- 跨团队合并：`strict` / `fast`
- 合并自愈：`--enable-merge-auto-rework` + `--max-merge-retries`
- 冲突路由标记：`[[merge:code-conflict]]` / `[[merge:api-conflict]]` / `[[merge:test-conflict]]`
- 冲突路由映射：可在 `config/team-runtime.yaml` 的 `merge_rework_routes` 自定义 team/role 路径
- 路由优先级：可在 `merge_rework_rules` 配置 marker 匹配顺序（priority 越小越先匹配）
- 条件字段：`required_risk_level` / `min_retry_round` / `max_team_load`（可选）
- 条件组合：`condition_mode: all|any`（默认 `all`）
- 表达式条件：`condition_expression` 支持 `risk==...`、`risk>=...`、`risk<=...`、`retry>=...`、`retry<=...`、`team_load<=...`、`team_load>=...`，并可用 `&&`/`||`
- 角色管理：`--enable-role-failover` + `--max-role-attempts`（或 profile 中配置）
- 团队管理：`--team-topology single|multi` + `--max-parallel-teams`
- 策略可通过 `--gate-policy`、`--arbiter-policy`、`--merge-policy` 或 `--profile-file` 动态切换

## 工作流配置

- 默认内置工作流来自 `src/workflow_config.rs`
- 也可通过 YAML 覆盖：

```bash
cargo run -- --workflow-file config/workflows.yaml run "你的需求" --workflow default
```

## 需求标记（高级）

可在 requirement 中嵌入控制标记：

- `[[decisions:rework,continue]]`：给 autonomy 流程注入决策序列
- `[[danger:true]]`：模拟危险变更，配合 rollback 测试
- `[[discuss:exhaust]]`：强制讨论到过期状态
- `[[discuss:escalate]]`：强制讨论升级并触发人工闸门
- `[[approve:all]]`：跳过阶段人工闸门（测试场景）

## 开发命令

```bash
cargo fmt --all
cargo test
cargo build
```

## 路线图

- 见 `ROADMAP.md`
