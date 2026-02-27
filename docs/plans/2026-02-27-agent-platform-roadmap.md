# AI Orchestrator 90-Day Roadmap (2026-02-27 -> 2026-05-28)

## 1) Product Direction (Do this, not another model)

Position the project as an **AI execution layer**:
- Input: user requirement
- Output: auditable delivery result
- Core value: higher delivery certainty, safer autonomy, enterprise governance

Moat:
1. Multi-agent supervisor loop (not single prompt chain)
2. Hard guardrails + rollback
3. Plugin/skills/knowledge expansion model
4. Multi-tenant policy and permission isolation

---

## 2) Milestones

### M1 (2026-02-27 ~ 2026-03-13): Safety Base
Goal: make autonomous execution controllable and recoverable.

Deliverables:
1. True git checkpoint/rollback path in autonomy loop
2. Non-bypassable execution guardrails (command/file policy)
3. Risk-Judge decision fully connected to rollback action
4. Regression tests for rollback + guard violations

Exit criteria:
- `rollback` decision always returns workspace to last stable checkpoint
- dangerous command/file operations are denied by default and logged

---

### M2 (2026-03-14 ~ 2026-03-27): Black-box Async API
Goal: make the orchestrator consumable as a product interface.

Deliverables:
1. `submit -> jobId`
2. `status(jobId)` with stage, progress, risk, pending gates
3. `result(jobId)` with strict JSON output contract
4. job persistence + retry + error codes

Exit criteria:
- API can run workflows without CLI coupling
- result schema validated before returning

---

### M3 (2026-03-28 ~ 2026-04-24): Plugin + Permission System
Goal: support generic core + vertical capability injection.

Deliverables:
1. Plugin manifest spec (tools, scopes, limits, trust level)
2. Adapter layer: local plugin + MCP/HTTP plugin
3. Default-deny policy engine + tenant grants
4. Audit trail for each plugin call

Exit criteria:
- no plugin executes without explicit permission grant
- all plugin invocations are traceable by job/tenant

---

### M4 (2026-04-25 ~ 2026-05-22): Multi-Tenant + Knowledge Packs
Goal: turn one engine into reusable product platform.

Deliverables:
1. Tenant workspace isolation
2. Tenant config: workflow profile, model profile, plugin set
3. Knowledge-pack binding (general + vertical packs)
4. Cost/rate/quotas per tenant

Exit criteria:
- tenants cannot access each other's files, plugins, or knowledge
- per-tenant policies enforceable at runtime

---

### M5 (2026-05-23 ~ 2026-05-28): Vertical Pilot Packaging
Goal: validate commercialization loop.

Deliverables:
1. Choose one vertical package (recommended: software delivery copilot)
2. Prebuilt workflow template + plugin bundle + knowledge pack
3. Demo pipeline with measurable success baseline

Exit criteria:
- one end-to-end scenario reaches stable production-style demo quality

---

## 3) KPIs (North Star)

1. End-to-end success rate (no manual takeover)
2. Rollback recovery success rate
3. High-risk action interception rate
4. Mean time to usable result
5. Cost per successful job

---

## 4) Immediate Build Order (start now)

P0:
1. `src/tools/git.ts` + `src/workflow/pipeline.ts`: checkpoint/rollback real implementation
2. `src/tools/execution-guard.ts` + `src/tools/shell.ts` + `src/tools/filesystem.ts`: hard guardrails
3. tests: rollback + policy denial + audit evidence

P1:
1. `src/api/*` + `src/jobs/*` + storage schema/repo: async job API
2. strict result schema + validator in pipeline end state
