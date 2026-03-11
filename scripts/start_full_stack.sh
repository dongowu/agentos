#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)

API_ADDR=${AGENTOS_API_LISTEN_ADDR:-127.0.0.1:18080}
CTRL_ADDR=${AGENTOS_CONTROL_ADDR:-127.0.0.1:15052}
WORKER_ADDR=${AGENTOS_WORKER_ADDR:-127.0.0.1:15051}
AUTH_TOKEN=${AGENTOS_AUTH_TOKEN:-dev-token}
AUTH_PRINCIPAL=${AGENTOS_AUTH_PRINCIPAL:-dev-user\|tenant-dev}
WORKER_ID=${AGENTOS_STACK_WORKER_ID:-local-worker}
HEARTBEAT_TIMEOUT=${AGENTOS_SCHEDULER_HEARTBEAT_TIMEOUT:-3s}
HEALTH_CHECK_INTERVAL=${AGENTOS_SCHEDULER_HEALTH_CHECK_INTERVAL:-250ms}
HEARTBEAT_INTERVAL_SECS=${AGENTOS_HEARTBEAT_INTERVAL_SECS:-1}
SMOKE_PROMPT=${AGENTOS_STACK_SMOKE_PROMPT:-echo hello}

STACK_DIR=""
BIN_DIR=""
LOG_DIR=""
GO_CACHE_DIR=""

CONTROLLER_PID=""
APISERVER_PID=""
WORKER_PID=""
WORKER_BIN=""
OSCTL_BIN=""

SMOKE_TEST=0
EXIT_AFTER_SMOKE=0
HELP_ONLY=0

usage() {
  cat <<EOF
Usage: $(basename "$0") [--smoke-test] [--exit-after-smoke]

Starts the local AgentOS stack:
  - controller
  - apiserver
  - Rust worker

Options:
  --smoke-test        Submit a sample task after startup
  --exit-after-smoke  Exit after the smoke test completes (implies --smoke-test)
  -h, --help          Show this help
EOF
}

log_info() {
  printf '[agentos-stack] %s\n' "$*" >&2
}

log_error() {
  printf '[agentos-stack] ERROR: %s\n' "$*" >&2
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --smoke-test)
        SMOKE_TEST=1
        ;;
      --exit-after-smoke)
        SMOKE_TEST=1
        EXIT_AFTER_SMOKE=1
        ;;
      -h|--help)
        HELP_ONLY=1
        usage
        return 0
        ;;
      *)
        log_error "unknown option: $1"
        usage >&2
        return 1
        ;;
    esac
    shift
  done
}

require_cmd() {
  local cmd=$1
  command -v "$cmd" >/dev/null 2>&1 || {
    log_error "missing required command: $cmd"
    return 1
  }
}

init_layout() {
  STACK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/agentos-local-stack.XXXXXX")
  BIN_DIR="$STACK_DIR/bin"
  LOG_DIR="$STACK_DIR/logs"
  GO_CACHE_DIR="${GOCACHE:-$STACK_DIR/go-build}"
  mkdir -p "$BIN_DIR" "$LOG_DIR" "$GO_CACHE_DIR"
}

wait_for_http() {
  local url=$1
  local attempts=${2:-60}
  local delay=${3:-1}
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
  local delay=${4:-1}
  local attempt
  for attempt in $(seq 1 "$attempts"); do
    if grep -Eq "$pattern" "$file" 2>/dev/null; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}

extract_task_id() {
  local output=$1
  sed -n 's/^task \(task-[0-9][0-9]*\) created.*/\1/p' <<<"$output"
}

json_field() {
  local key=$1
  sed -n "s/.*\"${key}\":\"\\([^\"]*\\)\".*/\\1/p"
}

cleanup() {
  local pid
  for pid in "$WORKER_PID" "$APISERVER_PID" "$CONTROLLER_PID"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
}

build_binaries() {
  require_cmd go
  require_cmd cargo
  require_cmd curl

  log_info "building Go binaries"
  GOCACHE="$GO_CACHE_DIR" go build -o "$BIN_DIR/controller" ./cmd/controller
  GOCACHE="$GO_CACHE_DIR" go build -o "$BIN_DIR/apiserver" ./cmd/apiserver
  GOCACHE="$GO_CACHE_DIR" go build -o "$BIN_DIR/osctl" ./cmd/osctl

  log_info "building Rust worker"
  (
    cd "$ROOT_DIR/runtime"
    cargo build -p agentos-worker >/dev/null
  )

  WORKER_BIN="$ROOT_DIR/runtime/target/debug/agentos-worker"
  OSCTL_BIN="$BIN_DIR/osctl"
}

start_controller() {
  log_info "starting controller on $CTRL_ADDR"
  AGENTOS_MODE=dev \
  AGENTOS_SCHEDULER_HEARTBEAT_TIMEOUT="$HEARTBEAT_TIMEOUT" \
  AGENTOS_SCHEDULER_HEALTH_CHECK_INTERVAL="$HEALTH_CHECK_INTERVAL" \
  GRPC_LISTEN_ADDR="$CTRL_ADDR" \
  "$BIN_DIR/controller" >"$LOG_DIR/controller.log" 2>&1 &
  CONTROLLER_PID=$!

  wait_for_log "$LOG_DIR/controller.log" 'controller started' 60 1 || {
    log_error "controller did not become ready"
    return 1
  }
}

start_apiserver() {
  log_info "starting apiserver on $API_ADDR"
  AGENTOS_MODE=dev \
  AGENTOS_CONTROL_PLANE_ADDR="$CTRL_ADDR" \
  AGENTOS_API_LISTEN_ADDR="$API_ADDR" \
  AGENTOS_AUTH_TOKENS="$AUTH_TOKEN=$AUTH_PRINCIPAL" \
  "$BIN_DIR/apiserver" >"$LOG_DIR/apiserver.log" 2>&1 &
  APISERVER_PID=$!

  wait_for_http "http://$API_ADDR/health" 60 1 || {
    log_error "apiserver health check did not become ready"
    return 1
  }
}

start_worker() {
  log_info "starting worker on $WORKER_ADDR"
  AGENTOS_RUNTIME=native \
  AGENTOS_WORKER_ID="$WORKER_ID" \
  AGENTOS_LISTEN_ADDR="$WORKER_ADDR" \
  AGENTOS_CONTROL_PLANE_ADDR="$CTRL_ADDR" \
  AGENTOS_HEARTBEAT_INTERVAL_SECS="$HEARTBEAT_INTERVAL_SECS" \
  "$WORKER_BIN" >"$LOG_DIR/worker.log" 2>&1 &
  WORKER_PID=$!

  wait_for_log "$LOG_DIR/worker.log" 'agentos-worker listening on' 60 1 || {
    log_error "worker did not become ready"
    return 1
  }
}

run_osctl_submit() {
  : "${OSCTL_BIN:?OSCTL_BIN must be set}"
  : "${API_ADDR:?API_ADDR must be set}"

  local prompt=$1
  local -a cmd=("$OSCTL_BIN" "--server" "http://$API_ADDR")
  if [[ -n "${AUTH_TOKEN:-}" ]]; then
    cmd+=("--token" "$AUTH_TOKEN")
  fi
  cmd+=("submit" "$prompt")
  "${cmd[@]}"
}

print_summary() {
  cat <<EOF

AgentOS local stack is running.

Addresses:
  controller: $CTRL_ADDR
  apiserver:  $API_ADDR
  worker:     $WORKER_ADDR

Auth:
  token:      $AUTH_TOKEN

Logs:
  $LOG_DIR/controller.log
  $LOG_DIR/apiserver.log
  $LOG_DIR/worker.log

Quick checks:
  curl http://$API_ADDR/health
  AGENTOS_SERVER_URL=http://$API_ADDR AGENTOS_AUTH_TOKEN=$AUTH_TOKEN go run ./cmd/osctl workers
  AGENTOS_SERVER_URL=http://$API_ADDR AGENTOS_AUTH_TOKEN=$AUTH_TOKEN go run ./cmd/osctl submit "echo hello"

Press Ctrl-C to stop all processes.
EOF
}

run_smoke_test() {
  log_info "running smoke test: $SMOKE_PROMPT"
  local output
  output=$(run_osctl_submit "$SMOKE_PROMPT" 2>&1)
  printf '%s\n' "$output"

  local task_id
  task_id=$(extract_task_id "$output")
  if [[ -z "$task_id" ]]; then
    log_error "could not parse task id from submit output"
    return 1
  fi

  wait_for_task_state "$task_id"
}

wait_for_task_state() {
  local task_id=$1
  local attempt
  for attempt in $(seq 1 120); do
    local response
    response=$(curl -fsS \
      -H "Authorization: Bearer $AUTH_TOKEN" \
      "http://$API_ADDR/v1/tasks/$task_id")

    local state
    state=$(printf '%s' "$response" | json_field state)
    if [[ "$state" == "succeeded" ]]; then
      log_info "smoke task succeeded: $task_id"
      return 0
    fi
    if [[ "$state" == "failed" ]]; then
      log_error "smoke task failed: $response"
      return 1
    fi
    sleep 0.5
  done

  log_error "smoke task did not complete in time: $task_id"
  return 1
}

monitor_stack() {
  while true; do
    if ! kill -0 "$CONTROLLER_PID" 2>/dev/null; then
      log_error "controller exited unexpectedly"
      return 1
    fi
    if ! kill -0 "$APISERVER_PID" 2>/dev/null; then
      log_error "apiserver exited unexpectedly"
      return 1
    fi
    if ! kill -0 "$WORKER_PID" 2>/dev/null; then
      log_error "worker exited unexpectedly"
      return 1
    fi
    sleep 1
  done
}

main() {
  parse_args "$@"
  if [[ "$HELP_ONLY" == "1" ]]; then
    return 0
  fi
  init_layout
  trap cleanup EXIT INT TERM

  build_binaries
  start_controller
  start_apiserver
  start_worker
  print_summary

  if [[ "$SMOKE_TEST" == "1" ]]; then
    run_smoke_test
  fi

  if [[ "$EXIT_AFTER_SMOKE" == "1" ]]; then
    return 0
  fi

  monitor_stack
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
