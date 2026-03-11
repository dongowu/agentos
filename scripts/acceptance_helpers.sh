#!/usr/bin/env bash

run_claw_dev() {
  : "${CLAW_BIN:?CLAW_BIN must be set}"
  : "${API_ADDR:?API_ADDR must be set}"

  local -a cmd=("$CLAW_BIN" "--server" "http://$API_ADDR")
  if [[ -n "${AUTH_TOKEN:-}" ]]; then
    cmd+=("--token" "$AUTH_TOKEN")
  fi
  cmd+=("dev" "$@")
  "${cmd[@]}"
}

run_osctl_workers() {
  : "${OSCTL_BIN:?OSCTL_BIN must be set}"
  : "${API_ADDR:?API_ADDR must be set}"

  local -a cmd=("$OSCTL_BIN" "--server" "http://$API_ADDR")
  if [[ -n "${AUTH_TOKEN:-}" ]]; then
    cmd+=("--token" "$AUTH_TOKEN")
  fi
  cmd+=("workers" "$@")
  "${cmd[@]}"
}

wait_for_claw_dev() {
  local attempts=$1
  local delay=$2
  shift 2

  local attempt
  for attempt in $(seq 1 "$attempts"); do
    if run_claw_dev "$@"; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}

expect_claw_dev_failure() {
  if run_claw_dev "$@"; then
    echo "expected claw dev to fail: $*" >&2
    return 1
  fi
  return 0
}

wait_for_claw_dev_failure() {
  local attempts=$1
  local delay=$2
  shift 2

  local attempt
  for attempt in $(seq 1 "$attempts"); do
    if ! run_claw_dev "$@"; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}

wait_for_osctl_workers() {
  local attempts=$1
  local delay=$2
  shift 2

  local attempt
  for attempt in $(seq 1 "$attempts"); do
    if run_osctl_workers "$@"; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}

expect_osctl_workers_failure() {
  if run_osctl_workers "$@"; then
    echo "expected osctl workers to fail: $*" >&2
    return 1
  fi
  return 0
}

wait_for_osctl_workers_failure() {
  local attempts=$1
  local delay=$2
  shift 2

  local attempt
  for attempt in $(seq 1 "$attempts"); do
    if ! run_osctl_workers "$@"; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}
