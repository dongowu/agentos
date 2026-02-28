#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[stage1] 1/3 run unit tests"
cargo test -q

echo "[stage1] 2/3 explain routing dry-run"
cargo run -- team-run "demo [[merge:api-conflict]]" --explain-routing --explain-retry-round 1 > /tmp/stage1_explain.json

echo "[stage1] validate explain output"
python3 - <<'PY'
import json
from pathlib import Path

path = Path('/tmp/stage1_explain.json')
data = json.loads(path.read_text())

assert data.get('selected_route', {}).get('route_name') == 'api-conflict', 'selected route must be api-conflict'
assert data.get('matched_rule', {}).get('route_key') == 'api-conflict', 'matched_rule route_key must be api-conflict'
assert isinstance(data.get('checks'), list) and data['checks'], 'checks must be a non-empty list'
assert any(item.get('matched') for item in data['checks']), 'at least one rule should match'
assert 'team_loads' in data and isinstance(data['team_loads'], dict), 'team_loads must be present'
assert 'max_risk_level' in data, 'max_risk_level must be present'
PY

echo "[stage1] 3/3 execute multi-team flow"
cargo run -- team-run "demo [[merge:api-conflict]]" --team-topology multi --merge-policy strict --enable-merge-auto-rework --max-merge-retries 2 --max-parallel 4 --max-parallel-teams 2 --profile-file config/team-runtime.yaml > /tmp/stage1_run.json

echo "[stage1] validate run output"
python3 - <<'PY'
import json
from pathlib import Path

path = Path('/tmp/stage1_run.json')
data = json.loads(path.read_text())

assert data.get('status') == 'Completed', 'team run status must be Completed'
assert isinstance(data.get('gates'), list) and len(data['gates']) == 4, 'must have 4 gates'
assert all(g.get('approved') for g in data['gates']), 'all gates must be approved'
assert isinstance(data.get('trace'), list) and data['trace'], 'trace must be non-empty'
assert isinstance(data.get('tasks'), list) and data['tasks'], 'tasks must be non-empty'
assert any(task.get('task_id') == 'merge_rework_api_1' for task in data['tasks']), 'must include merge_rework_api_1 task evidence'
assert any('merge auto-rework round 1' in line for line in data['trace']), 'trace must include merge auto-rework round'
PY

echo "[stage1] PASS"
echo "explain output: /tmp/stage1_explain.json"
echo "run output: /tmp/stage1_run.json"
