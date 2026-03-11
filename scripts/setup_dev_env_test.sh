#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/agentos-setup-dev-test.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

[[ -f "$ROOT_DIR/scripts/setup_dev_env.sh" ]] || {
  echo "missing script: $ROOT_DIR/scripts/setup_dev_env.sh" >&2
  exit 1
}

# shellcheck source=/dev/null
source "$ROOT_DIR/scripts/setup_dev_env.sh"

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

test_parse_args_supports_check_tests_verify_and_env_file() {
  CHECK_ONLY=0
  WITH_TESTS=0
  VERIFY_STACK=0
  ENV_FILE="$ROOT_DIR/.env.agentos.dev"

  parse_args --check-only --with-tests --verify-stack --env-file "$TMP_DIR/dev.env" || fail "parse_args should succeed"

  [[ "$CHECK_ONLY" == "1" ]] || fail "expected CHECK_ONLY=1"
  [[ "$WITH_TESTS" == "1" ]] || fail "expected WITH_TESTS=1"
  [[ "$VERIFY_STACK" == "1" ]] || fail "expected VERIFY_STACK=1"
  [[ "$ENV_FILE" == "$TMP_DIR/dev.env" ]] || fail "unexpected env file: $ENV_FILE"
}

test_write_env_file_writes_expected_exports() {
  BIN_DIR="$TMP_DIR/bin"
  ENV_FILE="$TMP_DIR/dev.env"
  mkdir -p "$BIN_DIR"

  write_env_file || fail "write_env_file should succeed"

  local content
  content=$(cat "$ENV_FILE")
  assert_contains "$content" "export PATH=\"$BIN_DIR:\$PATH\""
  assert_contains "$content" "export AGENTOS_MODE=dev"
  assert_contains "$content" "export AGENTOS_SERVER_URL=http://127.0.0.1:18080"
  assert_contains "$content" "export AGENTOS_AUTH_TOKEN=dev-token"
  assert_contains "$content" "export AGENTOS_AUTH_PRINCIPAL='dev-user|tenant-dev'"
}

test_main_help_exits_before_bootstrap() {
  local output
  output=$(bash -lc '
    set -Eeuo pipefail
    source "'"$ROOT_DIR"'/scripts/setup_dev_env.sh"
    check_required_commands() { echo check-called; }
    init_layout() { echo init-called; }
    warm_dependencies() { echo warm-called; }
    build_binaries() { echo build-called; }
    write_env_file() { echo env-called; }
    print_summary() { echo summary-called; }
    maybe_run_tests() { echo tests-called; }
    maybe_verify_stack() { echo verify-called; }
    main --help
  ' 2>&1) || fail "main --help should succeed"

  assert_contains "$output" "Usage:"
  [[ "$output" != *"check-called"* ]] || fail "--help should not continue into bootstrap"
}

test_parse_args_supports_check_tests_verify_and_env_file
test_write_env_file_writes_expected_exports
test_main_help_exits_before_bootstrap
echo "ok - setup_dev_env"
