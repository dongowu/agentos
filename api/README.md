# AgentOS API

Protobuf contracts shared between Go control plane and Rust runtime.

## Layout

- `agentos/v1/` - v1 API definitions
  - `task.proto` - Task, Plan, Action
  - `runtime.proto` - RuntimeService, ExecuteAction, StreamOutput

## Codegen

```bash
# Go (requires protoc, protoc-gen-go, protoc-gen-go-grpc)
make proto-gen
# or: protoc -I api/proto --go_out=api/gen --go_opt=module=github.com/dongowu/ai-orchestrator \
#        --go-grpc_out=api/gen --go-grpc_opt=module=github.com/dongowu/ai-orchestrator \
#        api/proto/agentos/v1/runtime.proto

# Rust (in runtime/)
cargo build
```
