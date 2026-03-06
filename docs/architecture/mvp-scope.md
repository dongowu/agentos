# AgentOS MVP Scope

## In Scope (Completed)

- Bootstrap the monorepo for Go and Rust development
- Define domain models for tasks, plans, actions, and events
- Define protobuf contracts between controller and worker
- Implement a Go control-plane skeleton with TaskEngine, Planner, SkillResolver
- Implement a Rust runtime skeleton (worker, sandbox, telemetry)
- Pluggable adapters: EventBus (memory, nats), TaskRepository (memory, postgres)
- Bootstrap wiring with config; default NATS + PostgreSQL
- CLI and HTTP API for task submission and status

## In Scope (Remaining)

- (Optional) NATS-based async dispatch for multi-worker scaling

## Out of Scope

- Browser automation
- Firecracker-based isolation
- Vector memory
- OAuth login flows
- Web3 signing and on-chain automation
- Production dashboard frontend

## MVP Success Criteria

An operator can submit one task, the control plane can turn it into one executable action, the runtime can execute it in a sandbox abstraction, and the final result can be persisted and streamed back.
