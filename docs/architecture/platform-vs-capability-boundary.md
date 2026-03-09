# Platform vs Capability Boundary

## Purpose

AgentOS should separate **platform concerns** from **agent capability concerns**.

This boundary matters because the long-term product is an agent platform, not a loose collection of agent tricks. If the boundary stays blurry, every new planner, tool, or vertical workflow will leak into core orchestration and make the system harder to govern and commercialize.

## Three Layers

| Layer | Responsibility | Typical buyer value |
|------|----------------|---------------------|
| Platform Layer | execution control, auth, policy, scheduling, audit, tenancy, persistence | trust, governance, operations |
| Capability Layer | planners, memory providers, tools, skills, provider integrations | usefulness, task quality, ecosystem breadth |
| Product Layer | CLI, API, console, templates, onboarding | usability and adoption |

## Platform Layer

The platform layer should own:

- access and authentication
- task lifecycle and orchestration
- worker registry and dispatch
- scheduling and queue contracts
- policy and approval hooks
- audit and telemetry models
- tenancy, org, quota, and persistence boundaries

This layer should avoid direct knowledge of vertical workflows or provider-specific logic.

## Capability Layer

The capability layer should own:

- planner implementations
- memory implementations
- tools and skill adapters
- browser / shell / git / http capabilities
- provider integrations
- vertical agent packs and templates

This layer should plug into the platform through stable interfaces.

## Product Layer

The product layer should own:

- `osctl` and `claw-cli`
- HTTP surface and public API ergonomics
- future operator console
- examples, starter templates, and onboarding flows

This layer should assemble the experience, not own core platform state transitions.

## Recommended Directory Direction

A future-friendly shape would be:

```text
internal/platform/
  access/
  auth/
  orchestration/
  scheduler/
  registry/
  policy/
  audit/
  telemetry/
  tenancy/
  persistence/

internal/capability/
  planner/
  memory/
  tool/
  skill/
  provider/
  agentpack/

internal/product/
  http/
  cli/
  console/
  sdk/
```

## Mapping From Current Structure

| Current area | Target direction |
|-------------|------------------|
| `internal/access` | `internal/platform/access` or `internal/product/http` depending on responsibility |
| `internal/orchestration` | `internal/platform/orchestration` |
| `internal/scheduler` | `internal/platform/scheduler` |
| `internal/policy` | `internal/platform/policy` |
| `internal/adapters/llm` | `internal/capability/provider` or `internal/capability/planner` |
| `internal/adapters/memory` | `internal/capability/memory` |
| `internal/tool` | `internal/capability/tool` |
| `internal/gateway` | `internal/product/http` |
| `internal/worker` + runtime crates | platform runtime boundary |

## Dependency Rules

- Platform code may depend on interfaces, not concrete provider logic.
- Capability code may depend on platform contracts, but not mutate platform state arbitrarily.
- Product code may compose platform and capability modules, but should not contain core orchestration logic.
- Enterprise add-ons should prefer extension points over forking the core execution model.

## Commercialization Implication

This boundary also protects the business model:

- the **platform core** can stay open and trusted
- the **capability ecosystem** can grow without destabilizing governance
- the **commercial layer** can focus on enterprise operations, identity, and management experience

## Migration Guidance

Do not do a large refactor immediately.

Instead:

1. classify new work by platform, capability, or product responsibility
2. keep new packages aligned to that boundary
3. move old packages opportunistically when adjacent work already touches them
4. keep public contracts stable while internals evolve

This keeps the architecture legible without pausing feature delivery.
