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
```

## HTTP API

启动 API 服务器：

```bash
cargo run -- serve --addr 127.0.0.1:3000
```

### 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/jobs` | 提交任务 |
| POST | `/jobs/work` | 处理队列 (body: usize) |
| GET | `/jobs/:job_id` | 查询任务状态 |
| GET | `/jobs/:job_id/result` | 获取任务结果 |
| POST | `/jobs/:job_id/resume` | 恢复暂停的任务 |
| GET | `/decisions` | 列出待审批决策 |
| POST | `/decisions/:decision_id/approve` | 批准决策 |
| POST | `/decisions/:decision_id/reject` | 拒绝决策 |
| GET | `/trace/:pipeline_id` | 获取管道追踪 |

### 示例

```bash
# 启动服务
cargo run -- serve &

# 提交任务
curl -X POST http://127.0.0.1:3000/jobs \
  -H "Content-Type: application/json" \
  -d '{"requirement": "实现支付重试机制", "workflow": "autonomy"}'

# 处理队列
curl -X POST http://127.0.0.1:3000/jobs/work -H "Content-Type: application/json" -d '8'

# 查询状态
curl http://127.0.0.1:3000/jobs/<job_id>

# 查看待审批
curl http://127.0.0.1:3000/decisions

# 批准决策
curl -X POST http://127.0.0.1:3000/decisions/<decision_id>/approve

# 恢复任务
curl -X POST http://127.0.0.1:3000/jobs/<job_id>/resume
```

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
