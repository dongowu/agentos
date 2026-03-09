#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/agentos-acceptance.XXXXXX")
BIN_DIR="$TMP_DIR/bin"
LOG_DIR="$TMP_DIR/logs"
mkdir -p "$BIN_DIR" "$LOG_DIR"

API_ADDR=${AGENTOS_API_LISTEN_ADDR:-127.0.0.1:18080}
CTRL_ADDR=${AGENTOS_ACCEPTANCE_CONTROL_ADDR:-127.0.0.1:15052}
WORKER_ADDR=${AGENTOS_ACCEPTANCE_WORKER_ADDR:-127.0.0.1:15051}
TASK_PROMPT=${AGENTOS_ACCEPTANCE_PROMPT:-echo acceptance-one then echo acceptance-two}
EXPECTED_ACTIONS=${AGENTOS_ACCEPTANCE_EXPECTED_ACTIONS:-2}
BRIDGE_CONTENT=${AGENTOS_ACCEPTANCE_BRIDGE_CONTENT:-bridge-acceptance}
BRIDGE_FILE=${AGENTOS_ACCEPTANCE_BRIDGE_FILE:-$TMP_DIR/bridge-acceptance.txt}
BRIDGE_PROMPT=${AGENTOS_ACCEPTANCE_BRIDGE_PROMPT:-write $BRIDGE_CONTENT to $BRIDGE_FILE then read $BRIDGE_FILE}
AUTH_TOKEN=${AGENTOS_ACCEPTANCE_AUTH_TOKEN:-acceptance-token}
AUTH_PRINCIPAL=${AGENTOS_ACCEPTANCE_AUTH_PRINCIPAL:-acceptance-user|tenant-acceptance}
GO_CACHE_DIR=${GOCACHE:-$TMP_DIR/go-build}

CONTROLLER_PID=""
APISERVER_PID=""
WORKER_PID=""
FAILED=0

cleanup() {
  local exit_code=$?
  if [[ $exit_code -ne 0 ]]; then
    FAILED=1
  fi

  for pid in "$WORKER_PID" "$APISERVER_PID" "$CONTROLLER_PID"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done

  if [[ $FAILED -ne 0 ]]; then
    echo
    echo "acceptance failed; logs follow"
    echo "--- controller.log ---"
    sed -n '1,240p' "$LOG_DIR/controller.log" 2>/dev/null || true
    echo "--- apiserver.log ---"
    sed -n '1,240p' "$LOG_DIR/apiserver.log" 2>/dev/null || true
    echo "--- worker.log ---"
    sed -n '1,240p' "$LOG_DIR/worker.log" 2>/dev/null || true
    echo "logs kept in: $LOG_DIR"
  else
    rm -rf "$TMP_DIR"
  fi

  exit "$exit_code"
}
trap cleanup EXIT INT TERM

wait_for_http() {
  local url=$1
  local attempts=${2:-60}
  local delay=${3:-0.5}
  local attempt
  for attempt in $(seq 1 "$attempts"); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}

wait_for_log() {
  local file=$1
  local pattern=$2
  local attempts=${3:-60}
  local delay=${4:-0.5}
  local attempt
  for attempt in $(seq 1 "$attempts"); do
    if grep -q "$pattern" "$file" 2>/dev/null; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}

json_field() {
  local key=$1
  sed -n "s/.*\"${key}\":\"\([^\"]*\)\".*/\1/p"
}

echo "[acceptance] building Go binaries"
GOCACHE="$GO_CACHE_DIR" go build -o "$BIN_DIR/controller" ./cmd/controller
GOCACHE="$GO_CACHE_DIR" go build -o "$BIN_DIR/apiserver" ./cmd/apiserver

echo "[acceptance] building Rust worker"
(
  cd "$ROOT_DIR/runtime"
  cargo build -p agentos-worker >/dev/null
)
WORKER_BIN="$ROOT_DIR/runtime/target/debug/agentos-worker"

echo "[acceptance] starting controller on $CTRL_ADDR"
AGENTOS_MODE=dev \
GRPC_LISTEN_ADDR="$CTRL_ADDR" \
"$BIN_DIR/controller" >"$LOG_DIR/controller.log" 2>&1 &
CONTROLLER_PID=$!

if ! wait_for_log "$LOG_DIR/controller.log" "controller started" 60 0.5; then
  echo "controller did not become ready"
  exit 1
fi

echo "[acceptance] starting apiserver on $API_ADDR"
AGENTOS_MODE=dev \
AGENTOS_CONTROL_PLANE_ADDR="$CTRL_ADDR" \
AGENTOS_API_LISTEN_ADDR="$API_ADDR" \
AGENTOS_AUTH_TOKENS="$AUTH_TOKEN=$AUTH_PRINCIPAL" \
"$BIN_DIR/apiserver" >"$LOG_DIR/apiserver.log" 2>&1 &
APISERVER_PID=$!

if ! wait_for_http "http://$API_ADDR/health" 60 0.5; then
  echo "apiserver health check did not become ready"
  exit 1
fi

echo "[acceptance] starting worker on $WORKER_ADDR"
AGENTOS_RUNTIME=native \
AGENTOS_LISTEN_ADDR="$WORKER_ADDR" \
AGENTOS_CONTROL_PLANE_ADDR="$CTRL_ADDR" \
AGENTOS_HEARTBEAT_INTERVAL_SECS=1 \
"$WORKER_BIN" >"$LOG_DIR/worker.log" 2>&1 &
WORKER_PID=$!

if ! wait_for_log "$LOG_DIR/worker.log" "registered with control plane" 60 0.5; then
  echo "worker did not register with controller"
  exit 1
fi

echo "[acceptance] submitting task through apiserver"
SUBMIT_RESPONSE=$(curl -fsS -X POST "http://$API_ADDR/v1/tasks" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"prompt\":\"$TASK_PROMPT\"}")
TASK_ID=$(printf '%s' "$SUBMIT_RESPONSE" | json_field task_id)
INITIAL_STATE=$(printf '%s' "$SUBMIT_RESPONSE" | json_field state)
if [[ -z "$TASK_ID" ]]; then
  echo "failed to parse task_id from response: $SUBMIT_RESPONSE"
  exit 1
fi

echo "[acceptance] task_id=$TASK_ID initial_state=$INITIAL_STATE"

FINAL_STATE=""
for _ in $(seq 1 120); do
  TASK_RESPONSE=$(curl -fsS -H "Authorization: Bearer $AUTH_TOKEN" "http://$API_ADDR/v1/tasks/$TASK_ID")
  FINAL_STATE=$(printf '%s' "$TASK_RESPONSE" | json_field state)
  if [[ "$FINAL_STATE" == "succeeded" ]]; then
    break
  fi
  if [[ "$FINAL_STATE" == "failed" ]]; then
    echo "task entered failed state: $TASK_RESPONSE"
    exit 1
  fi
  sleep 0.5
 done

if [[ "$FINAL_STATE" != "succeeded" ]]; then
  echo "task did not reach succeeded state; last_state=$FINAL_STATE"
  exit 1
fi

echo "[acceptance] verifying task audit API"
TASK_AUDIT=$(curl -fsS -H "Authorization: Bearer $AUTH_TOKEN" "http://$API_ADDR/v1/tasks/$TASK_ID/audit")
ACTION_COUNT=$(printf '%s' "$TASK_AUDIT" | python3 -c 'import json,sys; data=json.load(sys.stdin); records=data.get("records") or []; print(len(records))')
if [[ "$ACTION_COUNT" -lt "$EXPECTED_ACTIONS" ]]; then
  echo "expected at least $EXPECTED_ACTIONS audit records, got $ACTION_COUNT: $TASK_AUDIT"
  exit 1
fi
ACTION_ID=$(printf '%s' "$TASK_AUDIT" | python3 -c 'import json,sys; data=json.load(sys.stdin); records=data.get("records") or []; assert records, "no audit records returned"; print(records[-1]["action_id"])')
if [[ -z "$ACTION_ID" ]]; then
  echo "failed to parse action_id from task audit response: $TASK_AUDIT"
  exit 1
fi

echo "[acceptance] verifying action audit API for action_id=$ACTION_ID"
ACTION_AUDIT=$(curl -fsS -H "Authorization: Bearer $AUTH_TOKEN" "http://$API_ADDR/v1/tasks/$TASK_ID/actions/$ACTION_ID/audit")
ACTION_EXIT=$(printf '%s' "$ACTION_AUDIT" | python3 -c 'import json,sys; data=json.load(sys.stdin); print(data.get("exit_code", ""))')
if [[ "$ACTION_EXIT" != "0" ]]; then
  echo "unexpected action audit exit code: $ACTION_AUDIT"
  exit 1
fi

echo "[acceptance] verifying global audit API"
GLOBAL_AUDIT=$(curl -fsS -H "Authorization: Bearer $AUTH_TOKEN" "http://$API_ADDR/v1/audit?failed=false&limit=20")
GLOBAL_MATCHES=$(printf '%s' "$GLOBAL_AUDIT" | python3 -c 'import json,sys; data=json.load(sys.stdin); records=data.get("records") or []; task_id=sys.argv[1]; print(sum(1 for r in records if r.get("task_id") == task_id and r.get("tenant_id") == "tenant-acceptance"))' "$TASK_ID")
if [[ "$GLOBAL_MATCHES" -lt 1 ]]; then
  echo "expected global audit feed to include accepted tenant record for task $TASK_ID: $GLOBAL_AUDIT"
  exit 1
fi

echo "[acceptance] stopping worker to verify tool bridge without Rust runtime"
if [[ -n "$WORKER_PID" ]] && kill -0 "$WORKER_PID" 2>/dev/null; then
  kill "$WORKER_PID" 2>/dev/null || true
  wait "$WORKER_PID" 2>/dev/null || true
fi
WORKER_PID=""

echo "[acceptance] submitting bridge task through apiserver"
BRIDGE_RESPONSE=$(curl -fsS -X POST "http://$API_ADDR/v1/tasks" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"prompt\":\"$BRIDGE_PROMPT\"}")
BRIDGE_TASK_ID=$(printf '%s' "$BRIDGE_RESPONSE" | json_field task_id)
BRIDGE_INITIAL_STATE=$(printf '%s' "$BRIDGE_RESPONSE" | json_field state)
if [[ -z "$BRIDGE_TASK_ID" ]]; then
  echo "failed to parse bridge task_id from response: $BRIDGE_RESPONSE"
  exit 1
fi

echo "[acceptance] bridge_task_id=$BRIDGE_TASK_ID initial_state=$BRIDGE_INITIAL_STATE"
BRIDGE_FINAL_STATE=""
for _ in $(seq 1 120); do
  BRIDGE_TASK_RESPONSE=$(curl -fsS -H "Authorization: Bearer $AUTH_TOKEN" "http://$API_ADDR/v1/tasks/$BRIDGE_TASK_ID")
  BRIDGE_FINAL_STATE=$(printf '%s' "$BRIDGE_TASK_RESPONSE" | json_field state)
  if [[ "$BRIDGE_FINAL_STATE" == "succeeded" ]]; then
    break
  fi
  if [[ "$BRIDGE_FINAL_STATE" == "failed" ]]; then
    echo "bridge task entered failed state: $BRIDGE_TASK_RESPONSE"
    exit 1
  fi
  sleep 0.5
 done

if [[ "$BRIDGE_FINAL_STATE" != "succeeded" ]]; then
  echo "bridge task did not reach succeeded state; last_state=$BRIDGE_FINAL_STATE"
  exit 1
fi

if [[ ! -f "$BRIDGE_FILE" ]]; then
  echo "bridge file was not created: $BRIDGE_FILE"
  exit 1
fi
BRIDGE_FILE_CONTENT=$(cat "$BRIDGE_FILE")
if [[ "$BRIDGE_FILE_CONTENT" != "$BRIDGE_CONTENT" ]]; then
  echo "unexpected bridge file content: $BRIDGE_FILE_CONTENT"
  exit 1
fi

BRIDGE_AUDIT=$(curl -fsS -H "Authorization: Bearer $AUTH_TOKEN" "http://$API_ADDR/v1/tasks/$BRIDGE_TASK_ID/audit")
BRIDGE_LAST_WORKER=$(printf '%s' "$BRIDGE_AUDIT" | python3 -c 'import json,sys; data=json.load(sys.stdin); records=data.get("records") or []; assert records, "no bridge audit records returned"; print(records[-1].get("worker_id", ""))')
if [[ "$BRIDGE_LAST_WORKER" != "control-plane" ]]; then
  echo "expected bridge audit worker_id control-plane, got $BRIDGE_LAST_WORKER: $BRIDGE_AUDIT"
  exit 1
fi

echo "[acceptance] success"
echo "- controller: $CTRL_ADDR"
echo "- apiserver:  $API_ADDR"
echo "- worker:     stopped before bridge verification"
echo "- task_id:    $TASK_ID"
echo "- action_id:  $ACTION_ID"
echo "- final:      $FINAL_STATE"
echo "- action_cnt: $ACTION_COUNT"
echo "- bridge:     $BRIDGE_TASK_ID ($BRIDGE_FINAL_STATE via control-plane)"
echo "- evidence:   worker registered, authenticated task submission succeeded, multi-step plan completed, audit endpoints returned persisted records, and file/http-style actions still executed after the Rust worker was stopped"
