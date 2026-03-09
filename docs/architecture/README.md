# Architecture Docs Index

This folder describes the current platform architecture of AgentOS: the system shape, platform boundaries, execution flow, and the key subsystems that make up the open-source core.

## Read These First

If you want the shortest path into the architecture, start here:

- [Project README](../../README.md)
- [Documentation Guide](../README.md)
- [Architecture Overview](overview.md)
- [Platform vs Capability Boundary](platform-vs-capability-boundary.md)
- [Multiprocess Acceptance](multiprocess-acceptance.md)

## In This Folder

- [Architecture Overview](overview.md) — current open-source system shape and main components
- [Monorepo Structure](monorepo-structure.md) — how the repository is organized across API, controller, runtime, and docs
- [Platform vs Capability Boundary](platform-vs-capability-boundary.md) — what belongs in the open core versus future packaging
- [Pluggable Adapters](adapters.md) — adapter seams for scheduler, memory, vault, runtime, and transport integrations
- [Skill System](skill-system.md) — how skills, agent surfaces, and capability packaging are framed
- [Policy Engine](policy-engine.md) — enforcement and guardrail model around action execution
- [Multiprocess Acceptance](multiprocess-acceptance.md) — evidence and shape of the real multi-process path
- [MVP Scope](mvp-scope.md) — what the current open-source baseline intentionally includes

## Suggested Reading Order

1. [Architecture Overview](overview.md)
2. [Platform vs Capability Boundary](platform-vs-capability-boundary.md)
3. [Monorepo Structure](monorepo-structure.md)
4. [Policy Engine](policy-engine.md)
5. [Multiprocess Acceptance](multiprocess-acceptance.md)
6. [Pluggable Adapters](adapters.md)

## Maintenance Notes

Keep this folder focused on the current architecture of the open-source platform. Historical designs and target-state sketches belong under [`../concepts/`](../concepts/README.md).
