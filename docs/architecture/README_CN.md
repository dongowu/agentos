# 架构文档索引

这个目录描述 AgentOS 当前的架构形态：系统边界、平台分层、执行链路，以及组成开源底座的关键子系统。

## 建议先看这些

如果你想用最短路径理解当前架构，建议从这里开始：

- [项目 README](../../README.md)
- [文档导航](../README_CN.md)
- [架构概览](overview.md)
- [平台层与能力层边界](platform-vs-capability-boundary.md)
- [多进程验收说明](multiprocess-acceptance.md)

## 本目录内容

- [架构概览](overview.md) — 当前开源系统形态与主要组件
- [Monorepo 结构](monorepo-structure.md) — API、controller、runtime 与文档如何组织在同一仓库中
- [平台层与能力层边界](platform-vs-capability-boundary.md) — 什么属于开源核心，什么属于后续包装层
- [可插拔适配器](adapters.md) — scheduler、memory、vault、runtime、transport 等适配边界
- [技能系统](skill-system.md) — skills、agent surface 与能力封装如何定位
- [策略引擎](policy-engine.md) — action 执行前后的约束、校验与护栏
- [多进程验收说明](multiprocess-acceptance.md) — 真实多进程链路的证据与结构
- [MVP 范围](mvp-scope.md) — 当前开源基线刻意包含与不包含的内容

## 建议阅读顺序

1. [架构概览](overview.md)
2. [平台层与能力层边界](platform-vs-capability-boundary.md)
3. [Monorepo 结构](monorepo-structure.md)
4. [策略引擎](policy-engine.md)
5. [多进程验收说明](multiprocess-acceptance.md)
6. [可插拔适配器](adapters.md)

## 维护说明

这个目录应聚焦当前开源平台的实际架构。历史方案和目标态草图应放在 [`../concepts/`](../concepts/README_CN.md) 下。
