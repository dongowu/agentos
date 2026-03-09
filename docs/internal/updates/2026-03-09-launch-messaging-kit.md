# AgentOS Launch Messaging Kit

- Date: 2026-03-09
- Audience: developers, platform teams, OSS adopters
- Positioning: open-source agent execution platform / community infrastructure core

## Messaging Guardrails

Use these rules consistently in public communication:

- describe AgentOS as an **open-source agent execution platform**
- emphasize the **Go control plane + Rust runtime plane** architecture
- position the public repo as the **community core**
- talk about execution, scheduling, audit, replay, and telemetry
- avoid overselling enterprise or cloud capabilities that are not in the public repository today
- avoid calling the current release a fully finished end-user product

## One-Line Positioning

AgentOS is an open-source agent execution platform with a Go control plane and a Rust runtime plane.

## Short Positioning

AgentOS helps teams run AI agents as governed workloads instead of ad-hoc scripts. The open-source core provides execution, orchestration, scheduling, audit, replay, and telemetry primitives in a self-hosted package.

## GitHub Repository Description

Use this for the repository description field:

> Open-source agent execution platform with a Go control plane and a Rust runtime plane.

## GitHub Repository About / Tag Suggestions

Suggested topics:

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

## GitHub Pinned Project Blurb

Use this for a pinned repository or profile summary:

> AgentOS is an open-source agent execution platform for teams that want more than prompt demos. It provides a Go control plane, a Rust worker runtime, local and NATS-backed scheduling, audit and replay APIs, and SSE telemetry for running agents as governed workloads.

## GitHub Release Announcement

### Short Version

AgentOS `v0.1.1` is now available.

This is the open-source community core: a self-hosted agent execution platform with a Go control plane, Rust worker runtime, local and NATS-backed scheduling, audit/replay APIs, and SSE telemetry.

### Long Version

Today we are publishing AgentOS `v0.1.1`, an open-source agent execution platform built around a Go control plane and a Rust runtime plane.

The goal of this repository is not to present a polished end-user chat product. The goal is to expose the infrastructure layer that lets teams run agents as governed workloads: submit tasks, orchestrate execution, schedule work across workers, inspect audit records, replay completed runs, and stream task/action telemetry.

The current community core includes:

- self-hosted control plane binaries
- Rust worker execution runtime
- local and NATS-backed scheduling paths
- task audit, replay, and SSE telemetry APIs
- agent loop / tool-calling support
- Apache-2.0 licensing for the repository core

If you want to evaluate or extend an agent execution substrate instead of another demo wrapper, this release is for you.

## X / Twitter Post

### Version A — Short

We just open-sourced AgentOS `v0.1.1`.

AgentOS is an agent execution platform with a Go control plane and a Rust runtime plane.

The community core includes self-hosted execution, scheduling, audit/replay APIs, and SSE telemetry.

GitHub: https://github.com/dongowu/agentos

### Version B — Slightly More Technical

We just released AgentOS `v0.1.1` as open source.

It is a self-hosted agent execution platform with:
- Go control plane
- Rust worker runtime
- local + NATS-backed scheduling
- audit / replay APIs
- SSE telemetry
- agent loop / tool-calling support

GitHub: https://github.com/dongowu/agentos

## X / Twitter Thread

1. We just open-sourced AgentOS `v0.1.1`.

2. AgentOS is an agent execution platform with a Go control plane and a Rust runtime plane.

3. The public repository focuses on the community core: execution, orchestration, scheduling, audit, replay, and telemetry.

4. It already supports local and NATS-backed scheduling, a Rust worker runtime, SSE task/action streams, and API-level audit surfaces.

5. It also includes an agent loop / tool-calling path, so the project is not limited to a stub planner demo.

6. The intent is simple: make agents runnable as governed workloads instead of ad-hoc scripts.

7. Repo: https://github.com/dongowu/agentos

## Hacker News / Reddit Submission Title Ideas

- Show HN: AgentOS — an open-source agent execution platform in Go and Rust
- Show HN: AgentOS, a self-hosted control plane for running AI agents
- Open-source agent execution platform with audit, replay, and telemetry APIs

## Launch FAQ Answers

### What is AgentOS?

AgentOS is an open-source platform for running agent workloads with execution, scheduling, audit, replay, and telemetry built into the core.

### Is this a chat app?

No. The current repository is focused on the infrastructure layer, not a polished consumer-facing chat product.

### What is open source today?

The repository core: control plane, runtime baseline, scheduler baseline, orchestration contracts, audit/telemetry APIs, CLI, examples, and docs.

### What should not be over-promised yet?

Do not imply that the repository already includes a full enterprise console, hosted cloud, complete org governance, or finished business workflow products.
