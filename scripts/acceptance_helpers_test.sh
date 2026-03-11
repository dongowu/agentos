#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/agentos-acceptance-test.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

[[ -f "$ROOT_DIR/scripts/acceptance_helpers.sh" ]] || {
  echo "missing helper library: $ROOT_DIR/scripts/acceptance_helpers.sh" >&2
  exit 1
}

# shellcheck source=/dev/null
source "$ROOT_DIR/scripts/acceptance_helpers.sh"

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

make_fake_claw() {
  local fail_until=$1
  local script_path=$2
  cat >"$script_path" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
STATE_FILE=${STATE_FILE:?}
ARGS_FILE=${ARGS_FILE:?}
FAIL_UNTIL=${FAIL_UNTIL:?}
count=0
if [[ -f "$STATE_FILE" ]]; then
  count=$(cat "$STATE_FILE")
fi
count=$((count + 1))
printf '%s\n' "$count" >"$STATE_FILE"
printf '%s\n' "$*" >>"$ARGS_FILE"
if (( count <= FAIL_UNTIL )); then
  exit 1
fi
exit 0
EOF
  chmod +x "$script_path"
}

make_fake_claw_succeed_until() {
  local succeed_until=$1
  local script_path=$2
  cat >"$script_path" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
STATE_FILE=${STATE_FILE:?}
ARGS_FILE=${ARGS_FILE:?}
SUCCEED_UNTIL=${SUCCEED_UNTIL:?}
count=0
if [[ -f "$STATE_FILE" ]]; then
  count=$(cat "$STATE_FILE")
fi
count=$((count + 1))
printf '%s\n' "$count" >"$STATE_FILE"
printf '%s\n' "$*" >>"$ARGS_FILE"
if (( count <= SUCCEED_UNTIL )); then
  exit 0
fi
exit 1
EOF
  chmod +x "$script_path"
}

test_wait_for_claw_dev_retries_until_success() {
  local state_file="$TMP_DIR/retry-count.txt"
  local args_file="$TMP_DIR/retry-args.txt"
  local fake_claw="$TMP_DIR/fake-claw-retry.sh"
  make_fake_claw 2 "$fake_claw"

  export STATE_FILE="$state_file"
  export ARGS_FILE="$args_file"
  export FAIL_UNTIL=2
  export CLAW_BIN="$fake_claw"
  export API_ADDR="127.0.0.1:18080"
  export AUTH_TOKEN="acceptance-token"

  wait_for_claw_dev 5 0 --require-ready --require-capability native || fail "wait_for_claw_dev should have succeeded"

  [[ $(cat "$state_file") == "3" ]] || fail "expected 3 claw attempts"
  local last_args
  last_args=$(tail -n 1 "$args_file")
  assert_contains "$last_args" "--server http://127.0.0.1:18080"
  assert_contains "$last_args" "--token acceptance-token"
  assert_contains "$last_args" "dev --require-ready --require-capability native"
}

test_expect_claw_dev_failure_succeeds_when_probe_fails() {
  local state_file="$TMP_DIR/fail-count.txt"
  local args_file="$TMP_DIR/fail-args.txt"
  local fake_claw="$TMP_DIR/fake-claw-fail.sh"
  make_fake_claw 9 "$fake_claw"

  export STATE_FILE="$state_file"
  export ARGS_FILE="$args_file"
  export FAIL_UNTIL=9
  export CLAW_BIN="$fake_claw"
  export API_ADDR="127.0.0.1:18080"
  export AUTH_TOKEN="acceptance-token"

  expect_claw_dev_failure --require-ready --require-capability native || fail "expect_claw_dev_failure should treat non-zero claw exit as success"
  [[ $(cat "$state_file") == "1" ]] || fail "expected 1 claw attempt"
}

test_wait_for_claw_dev_failure_retries_until_failure() {
  local state_file="$TMP_DIR/failure-count.txt"
  local args_file="$TMP_DIR/failure-args.txt"
  local fake_claw="$TMP_DIR/fake-claw-failure.sh"
  make_fake_claw_succeed_until 2 "$fake_claw"

  export STATE_FILE="$state_file"
  export ARGS_FILE="$args_file"
  export SUCCEED_UNTIL=2
  export CLAW_BIN="$fake_claw"
  export API_ADDR="127.0.0.1:18080"
  export AUTH_TOKEN="acceptance-token"

  wait_for_claw_dev_failure 5 0 --require-ready --require-capability native || fail "wait_for_claw_dev_failure should have succeeded"

  [[ $(cat "$state_file") == "3" ]] || fail "expected 3 claw attempts before failure"
  local last_args
  last_args=$(tail -n 1 "$args_file")
  assert_contains "$last_args" "dev --require-ready --require-capability native"
}

test_wait_for_osctl_workers_retries_until_success() {
  local state_file="$TMP_DIR/osctl-retry-count.txt"
  local args_file="$TMP_DIR/osctl-retry-args.txt"
  local fake_osctl="$TMP_DIR/fake-osctl-retry.sh"
  make_fake_claw 2 "$fake_osctl"

  export STATE_FILE="$state_file"
  export ARGS_FILE="$args_file"
  export FAIL_UNTIL=2
  export OSCTL_BIN="$fake_osctl"
  export API_ADDR="127.0.0.1:18080"
  export AUTH_TOKEN="acceptance-token"

  wait_for_osctl_workers 5 0 --available --require-count 1 --require-worker worker-1 || fail "wait_for_osctl_workers should have succeeded"

  [[ $(cat "$state_file") == "3" ]] || fail "expected 3 osctl attempts"
  local last_args
  last_args=$(tail -n 1 "$args_file")
  assert_contains "$last_args" "--server http://127.0.0.1:18080"
  assert_contains "$last_args" "--token acceptance-token"
  assert_contains "$last_args" "workers --available --require-count 1 --require-worker worker-1"
}

test_expect_osctl_workers_failure_succeeds_when_probe_fails() {
  local state_file="$TMP_DIR/osctl-fail-count.txt"
  local args_file="$TMP_DIR/osctl-fail-args.txt"
  local fake_osctl="$TMP_DIR/fake-osctl-fail.sh"
  make_fake_claw 9 "$fake_osctl"

  export STATE_FILE="$state_file"
  export ARGS_FILE="$args_file"
  export FAIL_UNTIL=9
  export OSCTL_BIN="$fake_osctl"
  export API_ADDR="127.0.0.1:18080"
  export AUTH_TOKEN="acceptance-token"

  expect_osctl_workers_failure --available --require-count 1 || fail "expect_osctl_workers_failure should treat non-zero osctl exit as success"
  [[ $(cat "$state_file") == "1" ]] || fail "expected 1 osctl attempt"
}

test_wait_for_osctl_workers_failure_retries_until_failure() {
  local state_file="$TMP_DIR/osctl-failure-count.txt"
  local args_file="$TMP_DIR/osctl-failure-args.txt"
  local fake_osctl="$TMP_DIR/fake-osctl-failure.sh"
  make_fake_claw_succeed_until 2 "$fake_osctl"

  export STATE_FILE="$state_file"
  export ARGS_FILE="$args_file"
  export SUCCEED_UNTIL=2
  export OSCTL_BIN="$fake_osctl"
  export API_ADDR="127.0.0.1:18080"
  export AUTH_TOKEN="acceptance-token"

  wait_for_osctl_workers_failure 5 0 --available --require-count 1 || fail "wait_for_osctl_workers_failure should have succeeded"

  [[ $(cat "$state_file") == "3" ]] || fail "expected 3 osctl attempts before failure"
  local last_args
  last_args=$(tail -n 1 "$args_file")
  assert_contains "$last_args" "workers --available --require-count 1"
}

test_wait_for_claw_dev_retries_until_success
test_expect_claw_dev_failure_succeeds_when_probe_fails
test_wait_for_claw_dev_failure_retries_until_failure
test_wait_for_osctl_workers_retries_until_success
test_expect_osctl_workers_failure_succeeds_when_probe_fails
test_wait_for_osctl_workers_failure_retries_until_failure
echo "ok - acceptance_helpers"
