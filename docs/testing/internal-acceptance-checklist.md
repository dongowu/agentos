# Internal Acceptance Checklist (Stage 1)

This checklist is the release gate for the internal productivity phase.
Any item marked failed blocks merging feature branches into `main`.

## A. Baseline

- [ ] `./scripts/stage1_acceptance.sh` completes end-to-end with JSON assertions passing.
- [ ] `cargo test -q` passes locally.
- [ ] `cargo run -- team-run "demo [[merge:api-conflict]]" --explain-routing --explain-retry-round 1` returns valid JSON.
- [ ] `cargo run -- team-run "demo [[merge:api-conflict]]" --team-topology multi --merge-policy strict --enable-merge-auto-rework --max-merge-retries 2 --max-parallel 4 --max-parallel-teams 2 --profile-file config/team-runtime.yaml` completes successfully.

## B. Routing and Governance

- [ ] `--explain-routing` selected route matches expected rule marker.
- [ ] `checks` includes explicit failure reasons when a rule does not match.
- [ ] Disabled rule (`enabled: false`) is skipped and fallback route is used.
- [ ] Priority conflict is rejected by profile validation.

## C. Condition Expression Coverage

- [ ] Supports `&&`, `||`, `!`, and `(...)` grouping.
- [ ] Supports risk operators: `==`, `>=`, `<=`.
- [ ] Supports retry operators: `==`, `>=`, `<=`, `>`, `<`.
- [ ] Supports team load operators: `==`, `>=`, `<=`, `>`, `<`.
- [ ] Supports boolean literals: `true`, `false`.
- [ ] Supports whitespace around operators (`retry >= 1`, `risk == low`).

## D. Failure Handling

- [ ] Merge conflict without auto-rework escalates to human decision.
- [ ] Merge conflict with auto-rework can recover in expected scenarios.
- [ ] Trace includes route selection context (`marker`, `priority`, `mode`, `expr`).
- [ ] Trace includes fallback event when no rule matches.
- [ ] Stage-1 script asserts merge rework evidence (`merge_rework_api_1`) and auto-rework trace line.

## E. Branch Policy

- [ ] Work completed on feature branch (never direct on `main`).
- [ ] One unit per commit; each commit pushed before starting next unit.
- [ ] PR merged to `main` only after checklist A-D pass.

## Sign-off

- Date:
- Branch:
- Reviewer:
- Result: PASS / FAIL
