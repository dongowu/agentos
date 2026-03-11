# Multi-Process Acceptance

## Purpose

`scripts/acceptance.sh` validates the real three-process AgentOS control-plane loop:

- `controller` owns the shared worker registry
- `agentos-worker` registers itself and sends heartbeats
- `apiserver` uses the controller as a remote registry, selects the worker, and dispatches a real task

The script proves that registration, remote worker discovery, scheduler dispatch, authenticated task submission, and end-to-end task state updates work together across process boundaries.

It now also exercises two reliability-specific scenarios that sit on top of that baseline:

- a task submission that arrives before any worker is online, while scheduler submit retries are enabled
- a worker stop/start cycle where readiness degrades, the stopped worker becomes explicitly unschedulable, and the same worker id re-registers before a fresh task succeeds again

## What It Verifies

1. Controller starts and exposes the worker registry gRPC server.
2. Apiserver starts with remote-registry mode enabled.
3. Worker registers with the controller using `AGENTOS_CONTROL_PLANE_ADDR`.
4. An authenticated task submission to `POST /v1/tasks` reaches `succeeded`.
5. The default acceptance prompt expands into multiple actions, proving the control plane continues dispatching after the first action completes.
6. A temporary no-worker window does not immediately fail submission when the apiserver retry window is long enough for a worker to register.
7. `claw dev --require-ready --require-capability shell` reports degraded readiness before any worker is online, then flips healthy once capacity appears.
8. `osctl workers --require-available-count 1 --require-capability-available-count shell=1` and `osctl workers --available --require-count 1 --require-status-count online=1 --require-load-threshold 0.50 --require-capability-count shell=1 --require-capability-online-count shell=1 --require-worker acceptance-worker` both flip from failing to passing once the shared registry exposes a schedulable worker.
9. After the worker process is stopped and heartbeats expire, `claw dev --require-ready --require-capability shell` degrades again, `osctl workers --require-available-count 1 --require-capability-available-count shell=1` fails again, and `osctl workers --unschedulable-only --require-status-count offline=1 --require-capability-count shell=1 --require-capability-offline-count shell=1 --require-worker acceptance-worker` shows the offline worker snapshot.
10. After the worker process restarts, it re-registers with the controller and both CLI diagnostics plus a fresh task still succeed through the shared registry path.
11. Task and action audit APIs return persisted records for the multi-step run.
12. File-style bridge actions still succeed after the Rust worker is stopped, proving control-plane tool execution remains available.
13. The success path works without configuring `AGENTOS_WORKER_ADDR` on the apiserver, which means dispatch must happen via the shared registry + worker pool path.

## Run

```bash
./scripts/acceptance.sh
```

## Optional Overrides

- `AGENTOS_API_LISTEN_ADDR` defaults to `127.0.0.1:18080`
- `AGENTOS_ACCEPTANCE_CONTROL_ADDR` defaults to `127.0.0.1:15052`
- `AGENTOS_ACCEPTANCE_WORKER_ADDR` defaults to `127.0.0.1:15051`
- `AGENTOS_ACCEPTANCE_PROMPT` defaults to `echo acceptance-one then echo acceptance-two`
- `AGENTOS_ACCEPTANCE_EXPECTED_ACTIONS` defaults to `2`
- `AGENTOS_ACCEPTANCE_RESTART_PROMPT` defaults to `echo restart-acceptance`
- `AGENTOS_ACCEPTANCE_RESTART_EXPECTED_ACTIONS` defaults to `1`
- `AGENTOS_ACCEPTANCE_WORKER_ID` defaults to `acceptance-worker`
- `AGENTOS_ACCEPTANCE_TRANSIENT_WORKER_START_DELAY` defaults to `1`
- `AGENTOS_ACCEPTANCE_SUBMIT_RETRIES` defaults to `30`
- `AGENTOS_ACCEPTANCE_SUBMIT_RETRY_BACKOFF` defaults to `250ms`
- `AGENTOS_ACCEPTANCE_AUTH_TOKEN` defaults to `acceptance-token`
- `AGENTOS_ACCEPTANCE_REQUIRED_CAPABILITY` defaults to `shell`
- `AGENTOS_ACCEPTANCE_HEARTBEAT_TIMEOUT` defaults to `3s`
- `AGENTOS_ACCEPTANCE_HEALTH_CHECK_INTERVAL` defaults to `250ms`
- `AGENTOS_ACCEPTANCE_DEGRADE_ATTEMPTS` defaults to `40`
- `AGENTOS_ACCEPTANCE_DEGRADE_DELAY` defaults to `0.25`

## Notes

- The script builds fresh Go binaries and the Rust worker before starting processes.
- The apiserver is started with scheduler submit retries enabled so the transient no-worker scenario is intentional, not accidental.
- The controller is started with short heartbeat / health-check overrides by default so the stop-window degradation checks stay deterministic during local CI-style runs.
- The script also builds `claw` and `osctl`, then uses `claw dev --require-ready` plus `--require-capability` and `osctl workers --require-count` / `--require-available-count` / `--require-capability-available-count` / `--require-capability-online-count` / `--require-capability-offline-count` / `--require-load-threshold` / `--require-worker` / `--require-capability-count` / `--require-status-count` / `--unschedulable-only` as operator-facing proof that the public diagnostics surfaces match real schedulable capacity.
- The default script still uses `AGENTOS_MODE=dev`, so startup recovery of in-flight tasks across apiserver restarts is not covered here because memory persistence is not durable.
- On failure it prints controller, apiserver, and worker logs to aid debugging.
- On success it prints the delayed-start task id, restart task id, bridge task id, and observed action counts as evidence.
