# Integration Playbook For Engineering Systems

This guide is a lightweight contract for connecting AgentOS to the systems most R&D teams already use. The goal is practical adoption, not a new plugin framework.

## What The Contract Looks Like

Treat each integration as four simple parts:

1. A trigger from an existing system such as Git, CI, or an incident workflow.
2. A task payload that gives AgentOS the objective, repo context, and safety constraints.
3. An agent profile chosen from `examples/agents/` or a team-specific variant.
4. A result sink back into the source system, such as logs, comments, artifacts, or handoff notes.

Keep the contract explicit and boring. AgentOS should receive enough context to act, but not become the source of truth for repos, pipelines, or tickets.

## Minimum Task Payload

Use a stable payload shape when one system calls AgentOS:

```json
{
  "task": "review the release diff and call out rollback risk",
  "agent": "review-agent",
  "repository": "github.com/acme/payments",
  "ref": "refs/pull/184/head",
  "commit": "abc123",
  "context": {
    "pr_url": "https://github.com/acme/payments/pull/184",
    "ci_url": "https://ci.example/jobs/9912",
    "environment": "staging"
  },
  "constraints": {
    "change_window": "read-only",
    "allowed_tools": ["file.read", "git.diff", "browser.search"]
  }
}
```

Recommended conventions:

- Pass immutable repo coordinates such as commit SHA and ref.
- Pass system URLs so humans can trace the run.
- Pass environment and policy hints from the caller instead of hard-coding them in prompts.
- Keep credentials outside the payload and inject them through runtime secret management.

## Git And Pull Request Flows

Common ways to wire Git-hosted workflows without plugins:

- PR opened or updated -> CI job calls AgentOS with `review-agent`.
- Label applied such as `agentos/release-check` -> workflow triggers `release-agent`.
- Manual slash-command wrapper in your bot service -> submit a task with the chosen agent name.

Recommended outputs:

- PR comment with risks, missing tests, and follow-up items.
- CI artifact containing the full agent transcript or summarized findings.
- Audit link stored in the PR for traceability.

Keep write access narrow. Review paths should usually be read-only, with merge or tag creation left to existing release automation.

## CI And Verification Flows

Use CI as the orchestration layer that already knows repo checkout, branch metadata, and build state.

Suggested pattern:

1. CI checks out the repo and runs baseline verification.
2. CI submits an AgentOS task with the same commit SHA and artifact links.
3. AgentOS returns findings, remediation notes, or a release checklist.
4. CI publishes the result to logs, artifacts, or the originating PR.

This keeps AgentOS focused on governed execution while CI remains responsible for pipeline policy, retries, and gating.

## Release Operations

For release workflows, prefer an assistive contract:

- AgentOS gathers changelog inputs, upgrade notes, rollback concerns, and release evidence.
- CI or your release tool remains the actor that creates tags, publishes artifacts, or promotes environments.
- Human approval stays in the system your team already trusts.

The `release-agent.yaml` template is intentionally narrow enough to fit inside an existing release checklist rather than replace it.

## Runbooks And Incident Work

For ops-style workflows, map a runbook step to a task and keep the handoff explicit:

- Trigger from incident tooling, chatops, or an operator action.
- Use `ops-runbook-agent` with environment-specific constraints.
- Persist outputs as timeline notes, incident artifacts, or follow-up tasks.

Good runbook tasks are specific, such as "collect pod restart evidence in staging" or "verify error rate dropped after rollback," not open-ended debugging requests.

## Security And Governance Rules

Adopt these defaults early:

- Prefer per-agent allowlists over a broad global tool set.
- Inject secrets at runtime and never store them in agent YAML.
- Make read-only the default for review and diagnosis flows.
- Log the source trigger, repo, ref, and operator identity for every run.
- Return outputs to the originating system so reviews and incidents keep one canonical record.

## Rollout Plan For Teams

Start with one workflow from each category:

1. PR review assist using `review-agent`.
2. Release readiness check using `release-agent`.
3. Incident evidence collection using `ops-runbook-agent`.

Once those flows are stable, copy the templates and specialize them for each repository or environment instead of building a generic plugin layer first.
