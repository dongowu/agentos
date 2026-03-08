# Contributing to AgentOS

## Repository Layout

- `cmd/` - Go binary entry points (apiserver, controller, osctl)
- `internal/` - Private Go packages
  - `adapters/` - Pluggable implementations (messaging: memory, nats; persistence: memory, postgres)
  - `bootstrap/` - Config-based dependency wiring
- `pkg/` - Public Go packages (config, taskdsl, events)
- `api/proto/` - Protobuf contracts
- `runtime/crates/` - Rust worker, sandbox, telemetry
- `deploy/` - Docker Compose for local dev

## Local Setup

```bash
# Generate gRPC code (requires protoc, protoc-gen-go, protoc-gen-go-grpc)
make proto-gen

# Go
go build ./...
go test ./...

# Rust
cd runtime && cargo build && cargo test --workspace

# Run with in-memory adapters (no NATS/Postgres needed)
AGENTOS_MODE=dev go run ./cmd/osctl submit "echo hello"

# Run with default adapters (NATS + Postgres)
docker compose -f deploy/docker-compose.yml up -d
go run ./cmd/osctl submit "echo hello"
```

## Interface-First

Major subsystems use interfaces so implementations are pluggable. See README "Plug-in Boundaries" for details.

## Community Guidelines

- By participating, you agree to follow [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
- For security issues, do not open a public issue; follow [SECURITY.md](SECURITY.md).
- Unless otherwise stated, contributions to the open-source core are submitted under the repository license in [LICENSE](LICENSE).
