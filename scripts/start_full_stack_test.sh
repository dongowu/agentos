#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/agentos-start-stack-test.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

[[ -f "$ROOT_DIR/scripts/start_full_stack.sh" ]] || {
  echo "missing script: $ROOT_DIR/scripts/start_full_stack.sh" >&2
  exit 1
}

# shellcheck source=/dev/null
source "$ROOT_DIR/scripts/start_full_stack.sh"

fail() {
  echo "not ok - $1" >&2
  exit 1
}

assert_contains() {
  local haystack=$1
  local needle=$2
  if [[ "$haystack" != *"$needle"* ]]; then
    fail "expected [$haystack] to contain [$needle]"
  fi
}

make_fake_osctl() {
  local script_path=$1
  cat >"$script_path" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
ARGS_FILE=${ARGS_FILE:?}
printf '%s\n' "$*" >>"$ARGS_FILE"
EOF
  chmod +x "$script_path"
}

make_fake_osctl_submit_to_stderr() {
  local script_path=$1
  cat >"$script_path" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
printf 'task task-123456 created (state: running)\n' >&2
EOF
  chmod +x "$script_path"
}

test_run_osctl_submit_uses_server_token_and_prompt() {
  local args_file="$TMP_DIR/osctl-args.txt"
  local fake_osctl="$TMP_DIR/fake-osctl.sh"
  make_fake_osctl "$fake_osctl"

  export OSCTL_BIN="$fake_osctl"
  export API_ADDR="127.0.0.1:18080"
  export AUTH_TOKEN="dev-token"
  export ARGS_FILE="$args_file"

  run_osctl_submit "echo hello" || fail "run_osctl_submit should succeed"

  local args
  args=$(tail -n 1 "$args_file")
  assert_contains "$args" "--server http://127.0.0.1:18080"
  assert_contains "$args" "--token dev-token"
  assert_contains "$args" "submit echo hello"
}

test_parse_args_enables_smoke_and_exit_after_smoke() {
  SMOKE_TEST=0
  EXIT_AFTER_SMOKE=0

  parse_args --smoke-test --exit-after-smoke || fail "parse_args should succeed"

  [[ "$SMOKE_TEST" == "1" ]] || fail "expected SMOKE_TEST=1"
  [[ "$EXIT_AFTER_SMOKE" == "1" ]] || fail "expected EXIT_AFTER_SMOKE=1"
}

test_extract_task_id_from_submit_output() {
  local output='task task-123456 created (state: running)'
  local task_id
  task_id=$(extract_task_id "$output") || fail "extract_task_id should succeed"
  [[ "$task_id" == "task-123456" ]] || fail "unexpected task id: $task_id"
}

test_run_smoke_test_accepts_submit_output_from_stderr() {
  local fake_osctl="$TMP_DIR/fake-osctl-stderr.sh"
  make_fake_osctl_submit_to_stderr "$fake_osctl"

  OSCTL_BIN="$fake_osctl"
  API_ADDR="127.0.0.1:18080"
  AUTH_TOKEN="dev-token"
  SMOKE_PROMPT="echo hello"

  local waited_task_id=""
  wait_for_task_state() {
    waited_task_id=$1
  }

  run_smoke_test >/dev/null 2>&1 || fail "run_smoke_test should succeed when submit writes task id to stderr"
  [[ "$waited_task_id" == "task-123456" ]] || fail "expected waited task id to be parsed from stderr output"
}

test_main_help_exits_before_startup() {
  local output
  output=$(bash -lc '
    set -Eeuo pipefail
    source "'"$ROOT_DIR"'/scripts/start_full_stack.sh"
    init_layout() { echo init-called; }
    build_binaries() { echo build-called; }
    start_controller() { echo controller-called; }
    start_apiserver() { echo apiserver-called; }
    start_worker() { echo worker-called; }
    print_summary() { echo summary-called; }
    run_smoke_test() { echo smoke-called; }
    monitor_stack() { echo monitor-called; }
    main --help
  ' 2>&1) || fail "main --help should succeed"

  assert_contains "$output" "Usage:"
  [[ "$output" != *"init-called"* ]] || fail "--help should not continue into startup"
}

test_run_osctl_submit_uses_server_token_and_prompt
test_parse_args_enables_smoke_and_exit_after_smoke
test_extract_task_id_from_submit_output
test_run_smoke_test_accepts_submit_output_from_stderr
test_main_help_exits_before_startup
echo "ok - start_full_stack"
