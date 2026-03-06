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
TASK_PROMPT=${AGENTOS_ACCEPTANCE_PROMPT:-echo acceptance-ok}
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
  TASK_RESPONSE=$(curl -fsS "http://$API_ADDR/v1/tasks/$TASK_ID")
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

echo "[acceptance] success"
echo "- controller: $CTRL_ADDR"
echo "- apiserver:  $API_ADDR"
echo "- worker:     $WORKER_ADDR"
echo "- task_id:    $TASK_ID"
echo "- final:      $FINAL_STATE"
echo "- evidence:   worker registered and task succeeded through remote scheduler"
