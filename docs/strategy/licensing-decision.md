# AgentOS Licensing Decision

- Status: Adopted for repository core
- Date: 2026-03-08
- Audience: Founders, legal, product, platform maintainers

## TL;DR

Recommended licensing direction:

- **Open-source core**: `Apache-2.0`
- **Enterprise add-ons**: commercial license
- **Hosted cloud**: SaaS terms

This document records the adopted licensing direction for the repository core. The top-level `LICENSE` file now applies `Apache-2.0` to the open-source core in this repository. Enterprise add-ons and hosted cloud offerings remain outside that repository-core license boundary.

## Decision Goal

Choose a licensing model that:

- lowers enterprise adoption friction
- supports self-hosted deployments
- preserves room for enterprise and cloud monetization
- keeps the project credible as infrastructure, not a closed black box

## Recommended Decision

### Core

License the core repository under `Apache-2.0`.

This should cover the community edition platform surface:

- control plane binaries
- runtime plane baseline
- scheduler baseline
- task engine and orchestration contracts
- baseline policy and audit APIs
- SDK / CLI / examples / docs

### Enterprise Add-Ons

Keep enterprise-specific modules under a commercial license.

Examples:

- SSO / SAML / SCIM
- enterprise RBAC and org hierarchy
- advanced audit search and long retention
- approval workflows and compliance packs
- enterprise console and fleet administration
- support tooling and deployment accelerators

### Cloud Offering

Treat the hosted control plane as a managed service with separate SaaS terms.

## Why `Apache-2.0`

| Reason | Impact |
|--------|--------|
| Low friction | Easier evaluation by startups and enterprises |
| Infra familiarity | Fits buyer expectations for platform software |
| Ecosystem fit | Encourages adapters, providers, and integrations |
| Patent language | More comfortable for serious commercial adopters |
| Self-hosted friendliness | Supports the GTM motion for private deployments |

## Options Considered

### `MIT`

Pros:

- simplest and widely understood
- very low friction

Cons:

- weaker fit for infra positioning
- less helpful than `Apache-2.0` for some enterprise legal reviews

### `Apache-2.0`

Pros:

- strong fit for infrastructure products
- broadly accepted in enterprise contexts
- better patent posture than `MIT`

Cons:

- slightly more formal than `MIT`

### `AGPL`

Pros:

- stronger pressure against proprietary service forks

Cons:

- materially increases enterprise review friction
- can slow self-hosted evaluations
- may reduce ecosystem participation before the product has enough pull

### `BSL` / source-available models

Pros:

- stronger direct control over commercial use

Cons:

- weakens the open-core trust story
- less attractive to infra adopters who expect real OSS at the foundation

## Repository License State

The repository core now includes a top-level `LICENSE` file under `Apache-2.0`.

This applies to the open-source core contained in this repository. It does not automatically apply to future enterprise-only modules, commercial add-ons, or hosted service terms that may live outside the repository-core boundary.

## Decision Checklist

Completed or active repository actions:

1. Confirmed the intended model is a true OSS core rather than source-available.
2. Kept enterprise modules outside the repository-core license boundary.
3. Added `LICENSE` at repo root.
4. Updated README and contributing docs to reference the final repository-core license.
5. Documented where enterprise and cloud packaging diverge from the OSS core.

Remaining governance work:

1. Confirm contributor licensing process for future external contributors.
2. Decide whether a CLA or DCO is required before wider outside contribution.
3. Finalize how enterprise add-ons are packaged and distributed.

## Follow-Up Artifacts

Completed in this repository:

- added top-level `LICENSE`
- updated `README.md`
- updated `README_CN.md`
- added community/security documentation

Still recommended:

- add `NOTICE` if future attribution requirements appear
- document how commercial modules are packaged without confusing the OSS core
- publish an external packaging page when enterprise and cloud SKUs are customer-facing

## Recommended Next Step

Operate the repository core as `Apache-2.0`, keep enterprise and cloud packaging outside the repository-core boundary, and document contribution/licensing process before scaling outside community participation.
