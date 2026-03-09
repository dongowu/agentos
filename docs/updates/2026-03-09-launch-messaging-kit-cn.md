# AgentOS 首发传播素材包

- 日期：2026-03-09
- 面向对象：开发者、平台团队、开源早期采用者
- 对外定位：开源 Agent 执行平台 / 社区基础设施核心

## 对外表达原则

公开表达时建议始终保持以下边界：

- 把 AgentOS 描述为 **开源 Agent 执行平台**
- 强调 **Go 控制面 + Rust 运行时平面**
- 明确公开仓库当前是 **Community 核心**
- 重点讲执行、调度、审计、回放、遥测这些底座能力
- 不要提前过度承诺企业版或云服务里尚未公开的能力
- 不要把当前版本包装成已经完全成熟的终端产品

## 一句话定位

AgentOS 是一个采用 Go 控制面与 Rust 运行时平面的开源 Agent 执行平台。

## 短定位

AgentOS 帮助团队把 AI Agent 从临时脚本和 prompt demo 提升为可执行、可调度、可审计、可观察的工作负载。当前开源的 Community 核心提供 self-hosted 的执行、编排、调度、审计、回放和遥测能力。

## GitHub 仓库简介

可用于 GitHub 仓库简介字段：

> 开源 Agent 执行平台，采用 Go 控制面与 Rust 运行时平面。

## GitHub Topics 建议

建议主题：

- `agent`
- `ai`
- `llm`
- `agent-platform`
- `control-plane`
- `scheduler`
- `runtime`
- `golang`
- `rust`
- `open-source`

## GitHub 个人主页 / Pinned 项目文案

> AgentOS 是一个面向团队的开源 Agent 执行平台，不只是 prompt demo。它提供 Go 控制面、Rust worker runtime、本地与 NATS 调度、audit / replay API，以及 SSE telemetry，帮助团队把 Agent 作为可治理工作负载运行起来。

## 中文首发短帖

### 版本 A — 简短版

刚刚把 AgentOS `v0.1.1` 开源出来了。

它不是一个花哨的聊天壳，而是一个 Agent 执行平台：
- Go 控制面
- Rust worker runtime
- 本地 + NATS 调度
- audit / replay API
- SSE telemetry

仓库地址：https://github.com/dongowu/agentos

### 版本 B — 更完整一点

今天把 AgentOS `v0.1.1` 作为开源项目发出来了。

我想做的不是另一个 Agent demo，而是一个真正能承接 Agent 执行的底座：
- Go 控制面
- Rust worker runtime
- 任务编排与调度
- audit / replay API
- SSE 遥测流
- agent loop / tool-calling

当前公开的是 Community 核心：self-hosted execution substrate。
仓库地址：https://github.com/dongowu/agentos

## 中文长帖 / 公告稿

今天正式发布 AgentOS `v0.1.1`。

AgentOS 的定位不是“再做一个聊天产品”，而是把 Agent 当作一种可治理的工作负载来运行。为此，当前开源的 Community 核心提供了 Go 控制面、Rust worker runtime、本地与 NATS 调度路径，以及围绕任务执行的 audit、replay 和 SSE telemetry 能力。

这次开源更像是一个基础设施项目的首发版本，而不是一个已经完成所有产品包装的终端工具。它适合那些想 self-host 一个 Agent 执行底座、想理解执行/调度/审计如何打通、或者想在底层平台之上继续扩展适配器和工作流的团队。

当前仓库公开的重点很明确：Community 核心，也就是执行、编排、调度、审计、回放与遥测本身。企业级治理、托管云和更完整的商业化形态，不是这次首发要解决的重点。

如果你也在找一个不只是 prompt demo 的 Agent 底座，欢迎来看看这个项目。

仓库地址：https://github.com/dongowu/agentos

## 即刻 / 朋友圈简版

把 AgentOS 开源了，`v0.1.1`。

这是一个 Go 控制面 + Rust runtime 的 Agent 执行平台，重点不是聊天外壳，而是任务执行、调度、audit、replay 和 telemetry 这些基础设施能力。

仓库地址：https://github.com/dongowu/agentos

## 中文 FAQ 快速回答

### AgentOS 是什么？

AgentOS 是一个开源 Agent 执行平台，核心提供执行、调度、审计、回放和遥测能力。

### 它是聊天产品吗？

不是。当前仓库重点是底层基础设施，而不是面向终端用户的聊天产品。

### 现在开源的是什么？

现在公开的是 Community 核心：控制面、运行时、调度、编排契约、audit/telemetry API、CLI、示例和文档。

### 现在不该过度承诺什么？

不要过度承诺完整企业控制台、托管云、成熟组织治理能力，或者已经打磨好的行业工作流产品。
