#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/agentos-acceptance.XXXXXX")
BIN_DIR="$TMP_DIR/bin"
LOG_DIR="$TMP_DIR/logs"
mkdir -p "$BIN_DIR" "$LOG_DIR"
source "$ROOT_DIR/scripts/acceptance_helpers.sh"

API_ADDR=${AGENTOS_API_LISTEN_ADDR:-127.0.0.1:18080}
CTRL_ADDR=${AGENTOS_ACCEPTANCE_CONTROL_ADDR:-127.0.0.1:15052}
WORKER_ADDR=${AGENTOS_ACCEPTANCE_WORKER_ADDR:-127.0.0.1:15051}
TASK_PROMPT=${AGENTOS_ACCEPTANCE_PROMPT:-echo acceptance-one then echo acceptance-two}
EXPECTED_ACTIONS=${AGENTOS_ACCEPTANCE_EXPECTED_ACTIONS:-2}
RESTART_PROMPT=${AGENTOS_ACCEPTANCE_RESTART_PROMPT:-echo restart-acceptance}
RESTART_EXPECTED_ACTIONS=${AGENTOS_ACCEPTANCE_RESTART_EXPECTED_ACTIONS:-1}
BRIDGE_CONTENT=${AGENTOS_ACCEPTANCE_BRIDGE_CONTENT:-bridge-acceptance}
BRIDGE_FILE=${AGENTOS_ACCEPTANCE_BRIDGE_FILE:-$TMP_DIR/bridge-acceptance.txt}
BRIDGE_PROMPT=${AGENTOS_ACCEPTANCE_BRIDGE_PROMPT:-write $BRIDGE_CONTENT to $BRIDGE_FILE then read $BRIDGE_FILE}
AUTH_TOKEN=${AGENTOS_ACCEPTANCE_AUTH_TOKEN:-acceptance-token}
AUTH_PRINCIPAL=${AGENTOS_ACCEPTANCE_AUTH_PRINCIPAL:-acceptance-user|tenant-acceptance}
GO_CACHE_DIR=${GOCACHE:-$TMP_DIR/go-build}
WORKER_ID=${AGENTOS_ACCEPTANCE_WORKER_ID:-acceptance-worker}
TRANSIENT_WORKER_START_DELAY=${AGENTOS_ACCEPTANCE_TRANSIENT_WORKER_START_DELAY:-1}
SUBMIT_RETRIES=${AGENTOS_ACCEPTANCE_SUBMIT_RETRIES:-30}
SUBMIT_RETRY_BACKOFF=${AGENTOS_ACCEPTANCE_SUBMIT_RETRY_BACKOFF:-250ms}
REQUIRED_CAPABILITY=${AGENTOS_ACCEPTANCE_REQUIRED_CAPABILITY:-shell}
HEARTBEAT_TIMEOUT=${AGENTOS_ACCEPTANCE_HEARTBEAT_TIMEOUT:-3s}
HEALTH_CHECK_INTERVAL=${AGENTOS_ACCEPTANCE_HEALTH_CHECK_INTERVAL:-250ms}
DEGRADE_ATTEMPTS=${AGENTOS_ACCEPTANCE_DEGRADE_ATTEMPTS:-40}
DEGRADE_DELAY=${AGENTOS_ACCEPTANCE_DEGRADE_DELAY:-0.25}

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

wait_for_log_count() {
  local file=$1
  local pattern=$2
  local expected=$3
  local attempts=${4:-60}
  local delay=${5:-0.5}
  local attempt
  local count
  for attempt in $(seq 1 "$attempts"); do
    count=$(grep -c "$pattern" "$file" 2>/dev/null || true)
    if [[ "$count" -ge "$expected" ]]; then
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

json_prompt_payload() {
  local prompt=$1
  python3 -c 'import json,sys; print(json.dumps({"prompt": sys.argv[1]}))' "$prompt"
}

start_worker() {
  local delay=${1:-0}
  (
    sleep "$delay"
    exec env \
      AGENTOS_RUNTIME=native \
      AGENTOS_WORKER_ID="$WORKER_ID" \
      AGENTOS_LISTEN_ADDR="$WORKER_ADDR" \
      AGENTOS_CONTROL_PLANE_ADDR="$CTRL_ADDR" \
      AGENTOS_HEARTBEAT_INTERVAL_SECS=1 \
      "$WORKER_BIN"
  ) >>"$LOG_DIR/worker.log" 2>&1 &
  WORKER_PID=$!
}

stop_worker() {
  if [[ -n "$WORKER_PID" ]] && kill -0 "$WORKER_PID" 2>/dev/null; then
    kill "$WORKER_PID" 2>/dev/null || true
    wait "$WORKER_PID" 2>/dev/null || true
  fi
  WORKER_PID=""
}

submit_task() {
  local prompt=$1
  curl -fsS -X POST "http://$API_ADDR/v1/tasks" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H 'Content-Type: application/json' \
    -d "$(json_prompt_payload "$prompt")"
}

wait_for_task_state() {
  local task_id=$1
  local label=$2
  local final_state=""
  local task_response=""
  for _ in $(seq 1 120); do
    task_response=$(curl -fsS -H "Authorization: Bearer $AUTH_TOKEN" "http://$API_ADDR/v1/tasks/$task_id")
    final_state=$(printf '%s' "$task_response" | json_field state)
    if [[ "$final_state" == "succeeded" ]]; then
      printf '%s\n' "$task_response"
      return 0
    fi
    if [[ "$final_state" == "failed" ]]; then
      echo "$label entered failed state: $task_response"
      return 1
    fi
    sleep 0.5
  done
  echo "$label did not reach succeeded state; last_state=$final_state"
  return 1
}

echo "[acceptance] building Go binaries"
GOCACHE="$GO_CACHE_DIR" go build -o "$BIN_DIR/controller" ./cmd/controller
GOCACHE="$GO_CACHE_DIR" go build -o "$BIN_DIR/apiserver" ./cmd/apiserver
GOCACHE="$GO_CACHE_DIR" go build -o "$BIN_DIR/claw" ./cmd/claw-cli
GOCACHE="$GO_CACHE_DIR" go build -o "$BIN_DIR/osctl" ./cmd/osctl
CLAW_BIN="$BIN_DIR/claw"
OSCTL_BIN="$BIN_DIR/osctl"

echo "[acceptance] building Rust worker"
(
  cd "$ROOT_DIR/runtime"
  cargo build -p agentos-worker >/dev/null
)
WORKER_BIN="$ROOT_DIR/runtime/target/debug/agentos-worker"

echo "[acceptance] starting controller on $CTRL_ADDR"
AGENTOS_MODE=dev \
AGENTOS_SCHEDULER_HEARTBEAT_TIMEOUT="$HEARTBEAT_TIMEOUT" \
AGENTOS_SCHEDULER_HEALTH_CHECK_INTERVAL="$HEALTH_CHECK_INTERVAL" \
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
AGENTOS_SCHEDULER_SUBMIT_RETRIES="$SUBMIT_RETRIES" \
AGENTOS_SCHEDULER_SUBMIT_RETRY_BACKOFF="$SUBMIT_RETRY_BACKOFF" \
"$BIN_DIR/apiserver" >"$LOG_DIR/apiserver.log" 2>&1 &
APISERVER_PID=$!

if ! wait_for_http "http://$API_ADDR/health" 60 0.5; then
  echo "apiserver health check did not become ready"
  exit 1
fi

DIAGNOSTIC_ARGS=(--require-ready)
if [[ -n "$REQUIRED_CAPABILITY" ]]; then
  DIAGNOSTIC_ARGS+=(--require-capability "$REQUIRED_CAPABILITY")
fi
OSCTL_SUMMARY_ARGS=(--require-available-count 1)
OSCTL_AVAILABLE_ARGS=(--available --require-count 1 --require-status-count online=1 --require-load-threshold 0.50)
OSCTL_UNSCHEDULABLE_ARGS=(--unschedulable-only --require-count 1 --require-worker "$WORKER_ID" --require-status-count offline=1)
if [[ -n "$REQUIRED_CAPABILITY" ]]; then
  OSCTL_SUMMARY_ARGS+=(--require-capability-available-count "$REQUIRED_CAPABILITY=1")
  OSCTL_AVAILABLE_ARGS+=(--require-capability-count "$REQUIRED_CAPABILITY=1")
  OSCTL_AVAILABLE_ARGS+=(--require-capability-online-count "$REQUIRED_CAPABILITY=1")
  OSCTL_UNSCHEDULABLE_ARGS+=(--require-capability-count "$REQUIRED_CAPABILITY=1")
  OSCTL_UNSCHEDULABLE_ARGS+=(--require-capability-offline-count "$REQUIRED_CAPABILITY=1")
fi

echo "[acceptance] verifying claw dev reports degraded readiness before any worker is online"
if ! expect_claw_dev_failure "${DIAGNOSTIC_ARGS[@]}"; then
  echo "claw dev unexpectedly reported ready before any worker was online"
  exit 1
fi

echo "[acceptance] verifying osctl workers reports no schedulable workers before any worker is online"
if ! expect_osctl_workers_failure "${OSCTL_AVAILABLE_ARGS[@]}"; then
  echo "osctl workers unexpectedly reported schedulable workers before any worker was online"
  exit 1
fi

echo "[acceptance] verifying osctl workers summary reports no available capacity before any worker is online"
if ! expect_osctl_workers_failure "${OSCTL_SUMMARY_ARGS[@]}"; then
  echo "osctl workers summary unexpectedly reported available capacity before any worker was online"
  exit 1
fi

echo "[acceptance] submitting task through apiserver before any worker is online"
start_worker "$TRANSIENT_WORKER_START_DELAY"

SUBMIT_RESPONSE=$(submit_task "$TASK_PROMPT")
TASK_ID=$(printf '%s' "$SUBMIT_RESPONSE" | json_field task_id)
INITIAL_STATE=$(printf '%s' "$SUBMIT_RESPONSE" | json_field state)
if [[ -z "$TASK_ID" ]]; then
  echo "failed to parse task_id from response: $SUBMIT_RESPONSE"
  exit 1
fi

echo "[acceptance] task_id=$TASK_ID initial_state=$INITIAL_STATE"

if ! wait_for_log_count "$LOG_DIR/worker.log" "registered with control plane" 1 60 0.5; then
  echo "worker did not register with controller after delayed start"
  exit 1
fi

echo "[acceptance] waiting for claw dev readiness/capability assertions to recover"
if ! wait_for_claw_dev 60 0.5 "${DIAGNOSTIC_ARGS[@]}"; then
  echo "claw dev did not report ready capacity after worker registration"
  exit 1
fi

echo "[acceptance] waiting for osctl workers assertions to recover"
if ! wait_for_osctl_workers 60 0.5 "${OSCTL_AVAILABLE_ARGS[@]}" --require-worker "$WORKER_ID"; then
  echo "osctl workers did not report the expected schedulable worker after registration"
  exit 1
fi

echo "[acceptance] waiting for osctl workers summary availability assertions to recover"
if ! wait_for_osctl_workers 60 0.5 "${OSCTL_SUMMARY_ARGS[@]}"; then
  echo "osctl workers summary did not report available capacity after registration"
  exit 1
fi

TASK_RESPONSE=$(wait_for_task_state "$TASK_ID" "task")
FINAL_STATE=$(printf '%s' "$TASK_RESPONSE" | json_field state)
if [[ "$FINAL_STATE" != "succeeded" ]]; then
  echo "unexpected final state after delayed worker start: $FINAL_STATE"
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

stop_worker
echo "[acceptance] waiting for claw dev readiness assertions to degrade after worker stop"
if ! wait_for_claw_dev_failure "$DEGRADE_ATTEMPTS" "$DEGRADE_DELAY" "${DIAGNOSTIC_ARGS[@]}"; then
  echo "claw dev did not degrade after worker stop"
  exit 1
fi

echo "[acceptance] waiting for osctl workers to report no schedulable workers after worker stop"
if ! wait_for_osctl_workers_failure "$DEGRADE_ATTEMPTS" "$DEGRADE_DELAY" "${OSCTL_AVAILABLE_ARGS[@]}"; then
  echo "osctl workers still reported schedulable capacity after worker stop"
  exit 1
fi

echo "[acceptance] waiting for osctl workers summary to report no available capacity after worker stop"
if ! wait_for_osctl_workers_failure "$DEGRADE_ATTEMPTS" "$DEGRADE_DELAY" "${OSCTL_SUMMARY_ARGS[@]}"; then
  echo "osctl workers summary still reported available capacity after worker stop"
  exit 1
fi

echo "[acceptance] waiting for osctl workers to surface the stopped worker as unschedulable"
if ! wait_for_osctl_workers "$DEGRADE_ATTEMPTS" "$DEGRADE_DELAY" "${OSCTL_UNSCHEDULABLE_ARGS[@]}"; then
  echo "osctl workers did not surface the stopped worker as unschedulable"
  exit 1
fi

echo "[acceptance] restarting worker and verifying a fresh task still succeeds"
start_worker 0
if ! wait_for_log_count "$LOG_DIR/worker.log" "registered with control plane" 2 60 0.5; then
  echo "worker did not re-register with controller after restart"
  exit 1
fi

echo "[acceptance] re-checking claw dev readiness/capability assertions after worker restart"
if ! wait_for_claw_dev 60 0.5 "${DIAGNOSTIC_ARGS[@]}"; then
  echo "claw dev did not recover after worker restart"
  exit 1
fi

echo "[acceptance] re-checking osctl workers assertions after worker restart"
if ! wait_for_osctl_workers 60 0.5 "${OSCTL_AVAILABLE_ARGS[@]}" --require-worker "$WORKER_ID"; then
  echo "osctl workers did not recover after worker restart"
  exit 1
fi

echo "[acceptance] re-checking osctl workers summary availability assertions after worker restart"
if ! wait_for_osctl_workers 60 0.5 "${OSCTL_SUMMARY_ARGS[@]}"; then
  echo "osctl workers summary did not recover after worker restart"
  exit 1
fi

RESTART_RESPONSE=$(submit_task "$RESTART_PROMPT")
RESTART_TASK_ID=$(printf '%s' "$RESTART_RESPONSE" | json_field task_id)
RESTART_INITIAL_STATE=$(printf '%s' "$RESTART_RESPONSE" | json_field state)
if [[ -z "$RESTART_TASK_ID" ]]; then
  echo "failed to parse restart task_id from response: $RESTART_RESPONSE"
  exit 1
fi
echo "[acceptance] restart_task_id=$RESTART_TASK_ID initial_state=$RESTART_INITIAL_STATE"

RESTART_TASK_RESPONSE=$(wait_for_task_state "$RESTART_TASK_ID" "restart task")
RESTART_FINAL_STATE=$(printf '%s' "$RESTART_TASK_RESPONSE" | json_field state)
if [[ "$RESTART_FINAL_STATE" != "succeeded" ]]; then
  echo "restart task did not succeed; last_state=$RESTART_FINAL_STATE"
  exit 1
fi

RESTART_AUDIT=$(curl -fsS -H "Authorization: Bearer $AUTH_TOKEN" "http://$API_ADDR/v1/tasks/$RESTART_TASK_ID/audit")
RESTART_ACTION_COUNT=$(printf '%s' "$RESTART_AUDIT" | python3 -c 'import json,sys; data=json.load(sys.stdin); records=data.get("records") or []; print(len(records))')
if [[ "$RESTART_ACTION_COUNT" -lt "$RESTART_EXPECTED_ACTIONS" ]]; then
  echo "expected at least $RESTART_EXPECTED_ACTIONS restart audit records, got $RESTART_ACTION_COUNT: $RESTART_AUDIT"
  exit 1
fi
RESTART_LAST_WORKER=$(printf '%s' "$RESTART_AUDIT" | python3 -c 'import json,sys; data=json.load(sys.stdin); records=data.get("records") or []; assert records, "no restart audit records returned"; print(records[-1].get("worker_id", ""))')
if [[ "$RESTART_LAST_WORKER" != "$WORKER_ID" ]]; then
  echo "expected restart audit worker_id $WORKER_ID, got $RESTART_LAST_WORKER: $RESTART_AUDIT"
  exit 1
fi

echo "[acceptance] stopping worker to verify tool bridge without Rust runtime"
stop_worker

echo "[acceptance] submitting bridge task through apiserver"
BRIDGE_RESPONSE=$(submit_task "$BRIDGE_PROMPT")
BRIDGE_TASK_ID=$(printf '%s' "$BRIDGE_RESPONSE" | json_field task_id)
BRIDGE_INITIAL_STATE=$(printf '%s' "$BRIDGE_RESPONSE" | json_field state)
if [[ -z "$BRIDGE_TASK_ID" ]]; then
  echo "failed to parse bridge task_id from response: $BRIDGE_RESPONSE"
  exit 1
fi

echo "[acceptance] bridge_task_id=$BRIDGE_TASK_ID initial_state=$BRIDGE_INITIAL_STATE"
BRIDGE_TASK_RESPONSE=$(wait_for_task_state "$BRIDGE_TASK_ID" "bridge task")
BRIDGE_FINAL_STATE=$(printf '%s' "$BRIDGE_TASK_RESPONSE" | json_field state)
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
echo "- restart:    $RESTART_TASK_ID ($RESTART_FINAL_STATE via $RESTART_LAST_WORKER)"
echo "- bridge:     $BRIDGE_TASK_ID ($BRIDGE_FINAL_STATE via control-plane)"
echo "- evidence:   submit survived an initial no-worker window, the same worker id re-registered after restart and handled a fresh task, audit endpoints returned persisted records, and file/http-style actions still executed after the Rust worker was stopped"
