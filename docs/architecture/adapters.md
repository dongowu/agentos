# Pluggable Adapter Architecture

AgentOS uses a pluggable architecture for messaging and persistence. Interfaces are defined in core packages; implementations live in `internal/adapters/`.

## Overview

| Interface        | Default Adapter | Dev Adapter | Package                          |
|-----------------|-----------------|-------------|----------------------------------|
| `EventBus`      | NATS JetStream  | memory      | `internal/adapters/messaging/*`  |
| `TaskRepository`| PostgreSQL      | memory      | `internal/adapters/persistence/*`|

## Configuration

`pkg/config` defines adapter selection:

```go
// config.Default() - production: nats + postgres
// config.Dev()     - development: memory for both
```

Environment override:

- `AGENTOS_MODE=dev` → use memory adapters (no external services)
- Otherwise → use config.Default() (NATS + Postgres)

## Bootstrap

`internal/bootstrap` wires adapters:

```go
app, err := bootstrap.FromEnv(ctx)
// or
app, err := bootstrap.New(ctx, config.Dev())
```

## Adding an Adapter

1. Implement the interface in `internal/adapters/<layer>/<name>/`.
2. Add a case in `bootstrap.newEventBus` or `bootstrap.newTaskRepository`.
3. Extend `pkg/config` if new config fields are needed.

## Adapter Details

### EventBus (messaging)

- **memory**: In-process pub/sub. No external deps.
- **nats**: NATS JetStream. Requires `nats://host:4222`. Auto-creates stream.

### TaskRepository (persistence)

- **memory**: Map-backed. No external deps.
- **postgres**: pgx + JSONB for Plan. Requires DSN. Auto-migrates `tasks` table.
