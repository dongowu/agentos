#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[stage1] 1/3 run unit tests"
cargo test -q

echo "[stage1] 2/3 explain routing dry-run"
cargo run -- team-run "demo [[merge:api-conflict]]" --explain-routing --explain-retry-round 1 > /tmp/stage1_explain.json

echo "[stage1] 3/3 execute multi-team flow"
cargo run -- team-run "demo [[merge:api-conflict]]" --team-topology multi --merge-policy strict --enable-merge-auto-rework --max-merge-retries 2 --max-parallel 4 --max-parallel-teams 2 --profile-file config/team-runtime.yaml > /tmp/stage1_run.json

echo "[stage1] PASS"
echo "explain output: /tmp/stage1_explain.json"
echo "run output: /tmp/stage1_run.json"
