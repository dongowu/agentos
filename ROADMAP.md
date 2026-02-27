# AI Orchestrator Roadmap

> Last updated: 2026-02-27

## 产品目标

构建一个通用 Agent 执行内核：
- **输入**：用户需求
- **输出**：可验证结果（产物、轨迹、审批记录、回滚证据）
- **形态**：黑盒体验 + 可插拔能力（skills / plugins / knowledge）

---

## 当前已完成（基线）

- Rust CLI + SQLite 作业系统
- 多工作流编排（`mvp` / `default` / `autonomy`）
- Human Gate 审批流（pending / approve / reject）
- `resume` 续跑（从暂停点继续）
- 讨论会话记录持久化（conversation + message）
- `trace` 全链路追踪
- Git checkpoint / rollback
- Guardrail（shell + filesystem）
- 轻量 RAG 检索上下文

---

## 里程碑

## M1（2026-03-01 ~ 2026-03-14）：Runtime API 化

目标：从 CLI 工具进化到可集成服务。

交付：
- HTTP API：`submit/status/result/trace/resume`
- 统一 JSON Schema（入参与出参）
- 错误码与审计字段标准化

验收：
- 不依赖 CLI 也可完整跑通任务生命周期

## M2（2026-03-15 ~ 2026-03-31）：Plugin / Skill 接入层

目标：通用内核 + 可扩展能力。

交付：
- Plugin Manifest（工具声明、权限、限流、信任级别）
- 本地插件 + HTTP/MCP 插件适配器
- 默认拒绝策略（default deny）

验收：
- 所有插件调用可被策略拦截、可被 trace 回放

## M3（2026-04-01 ~ 2026-04-21）：Knowledge & Memory

目标：从“单次执行”走向“上下文持续可用”。

交付：
- 知识库抽象层（本地文件 / 向量库）
- Chunking / 检索策略可配置
- 任务级短期记忆 + 项目级长期记忆

验收：
- 多次任务间可复用历史知识，结果质量稳定提升

## M4（2026-04-22 ~ 2026-05-12）：并发 Agent Team

目标：多 Agent 并发协作可控可收敛。

交付：
- 并发调度器（分支任务 + 合并策略）
- 冲突检测与自动重试
- 收敛策略（vote/critic/supervisor）

验收：
- 在 2+ 独立子任务场景中，端到端耗时显著下降

## M5（2026-05-13 ~ 2026-05-31）：生产化与垂直化打包

目标：可直接落地一个垂直场景。

交付：
- 多租户隔离（配置、数据、插件权限）
- 指标监控（成功率、回滚率、成本、时延）
- 一个可交付的垂直包（如软件研发交付 Agent）

验收：
- 形成“开箱即用”的垂直模板（workflow + plugins + knowledge）

---

## 关键指标（KPI）

- 任务完成率（无需人工接管）
- 高风险动作拦截率
- 回滚成功率
- 平均交付时长（MTTR）
- 单任务成本（token + tool）

---

## 紧接着要做（Next 2 Weeks）

1. 增加 API server（axum）并对齐 CLI 能力
2. 抽象 model provider（OpenAI/Claude/Gemini）
3. 实现 plugin manifest + policy engine 最小可用版
4. 为 `trace` 增加时间线视图（stage/decision/message）
