# Multi-Process Acceptance

## Purpose

`scripts/acceptance.sh` validates the real three-process AgentOS control-plane loop:

- `controller` owns the shared worker registry
- `agentos-worker` registers itself and sends heartbeats
- `apiserver` uses the controller as a remote registry, selects the worker, and dispatches a real task

The script proves that registration, remote worker discovery, scheduler dispatch, and end-to-end task state updates work together across process boundaries.

## What It Verifies

1. Controller starts and exposes the worker registry gRPC server.
2. Apiserver starts with remote-registry mode enabled.
3. Worker registers with the controller using `AGENTOS_CONTROL_PLANE_ADDR`.
4. A task submitted to `POST /v1/tasks` reaches `succeeded`.
5. The success path works without configuring `AGENTOS_WORKER_ADDR` on the apiserver, which means dispatch must happen via the shared registry + worker pool path.

## Run

```bash
./scripts/acceptance.sh
```

## Optional Overrides

- `AGENTOS_API_LISTEN_ADDR` defaults to `127.0.0.1:18080`
- `AGENTOS_ACCEPTANCE_CONTROL_ADDR` defaults to `127.0.0.1:15052`
- `AGENTOS_ACCEPTANCE_WORKER_ADDR` defaults to `127.0.0.1:15051`
- `AGENTOS_ACCEPTANCE_PROMPT` defaults to `echo acceptance-ok`

## Notes

- The script builds fresh Go binaries and the Rust worker before starting processes.
- On failure it prints controller, apiserver, and worker logs to aid debugging.
- On success it prints the task id and final succeeded state as evidence.
