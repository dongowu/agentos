# Software Team Agent Templates

These YAML files are copy-paste-ready starter profiles for common R&D workflows.

## Included Templates

- `coding-agent.yaml` - implementation and refactoring tasks with file, git, and shell access
- `review-agent.yaml` - PR review, risk spotting, and change explanation with read-only style defaults
- `release-agent.yaml` - release checklist, changelog drafting, and verification steps
- `ops-runbook-agent.yaml` - runbook execution, incident triage, and operational follow-ups

## How To Use

1. Copy the closest template into your own agent config directory.
2. Change `model`, `tools`, and `policy` to match your environment.
3. Keep the workflow short and specific to the job you want the agent to perform.
4. Validate changes with `go test ./internal/agent`.

## Practical Guidance

- Start with the smallest tool allowlist that still lets the workflow succeed.
- Keep release and ops agents narrower than coding agents.
- Treat these files as examples of team conventions, not a fixed product taxonomy.

For integration guidance, see `docs/guides/integration-playbook.md`.
