# AgentOS Monorepo 最终版结构

> 目标：10 万行代码依然不乱，适合 Go + Rust 大型开源项目。

## 设计原则

1. **按领域分层**：access、orchestration、policy、skills 等独立
2. **internal 不对外**：仅 cmd、pkg、api、sdk 可被外部引用
3. **适配器可插拔**：messaging、persistence、runtime 在 adapters 下
4. **Rust 与 Go 边界清晰**：api/proto 为唯一跨语言契约

---

## 完整目录结构

```
agentos/
├── api/
│   ├── proto/                    # Protobuf 定义
│   │   └── agentos/v1/
│   │       ├── runtime.proto
│   │       ├── task.proto
│   │       └── events.proto
│   ├── gen/                       # 生成代码（git 可追踪或 make 生成）
│   │   └── agentos/v1/
│   └── openapi/                   # OpenAPI 3.0（未来）
│
├── cmd/
│   ├── apiserver/                 # HTTP + WebSocket 入口
│   ├── controller/                # 编排循环、任务调度
│   └── osctl/                     # CLI
│
├── internal/
│   ├── access/                    # Access System
│   │   ├── api_impl.go            # TaskSubmissionAPI 实现
│   │   ├── http/                  # HTTP 路由、中间件
│   │   ├── cli/                   # Cobra 命令
│   │   └── auth/                  # AuthProvider（未来）
│   │
│   ├── orchestration/             # Agent Brain + Task Engine
│   │   ├── contracts.go           # Planner, TaskEngine, SkillResolver 接口
│   │   ├── engine_impl.go         # TaskEngine 实现
│   │   ├── engine_impl_test.go
│   │   ├── stub_planner.go        # 测试/MVP 回退 Planner
│   │   ├── stub_skill_resolver.go  # 测试用简单 Resolver
│   │   └── planner/               # 未来：更多 Planner 适配器
│   │       ├── openai/
│   │       ├── claude/
│   │       └── local/
│   │
│   ├── policy/                    # Policy Engine（已实现）
│   │   ├── engine.go
│   │   ├── checker.go
│   │   └── loader.go
│   │
│   ├── skills/                   # Skill System（已实现基础注册表与 resolver）
│   │   ├── registry.go
│   │   ├── resolver.go
│   │   └── builtin.go
│   │
│   ├── broker/                   # Runtime Broker（设计稿，现由 scheduler/worker 承担主链路）
│   │   ├── scheduler.go
│   │   └── worker_pool.go
│   │
│   ├── messaging/                # EventBus 接口
│   │   └── event_bus.go
│   │
│   ├── persistence/              # TaskRepository 接口
│   │   └── task_repository.go
│   │
│   ├── adapter/                  # 适配器注册、工厂
│   │   └── registry.go
│   │
│   ├── adapters/                 # 可插拔实现
│   │   ├── defaults/             # 默认加载（import side-effect）
│   │   ├── messaging/
│   │   │   ├── memory/
│   │   │   └── nats/
│   │   ├── persistence/
│   │   │   ├── memory/
│   │   │   └── postgres/
│   │   └── runtimeclient/
│   │       ├── grpc.go
│   │       └── stub.go
│   │
│   ├── runtimeclient/            # ExecutorClient 接口与封装
│   │   ├── client.go
│   │   └── stub.go
│   │
│   └── bootstrap/                # 依赖装配
│       └── bootstrap.go
│
├── pkg/
│   ├── config/                   # 配置、适配器选择
│   │   └── config.go
│   ├── taskdsl/                  # 领域模型（可被 SDK 引用）
│   │   ├── task.go
│   │   ├── plan.go
│   │   └── action.go
│   └── events/                   # 领域事件
│       └── task_events.go
│
├── runtime/                      # Rust 执行面
│   ├── Cargo.toml
│   └── crates/
│       ├── worker/               # gRPC 服务、执行入口
│       │   ├── src/
│       │   │   ├── lib.rs
│       │   │   ├── grpc.rs
│       │   │   └── service.rs
│       │   └── build.rs
│       ├── sandbox/              # 隔离抽象
│       │   └── src/
│       │       ├── lib.rs
│       │       ├── provider.rs
│       │       └── docker.rs
│       ├── executor/             # 命令执行（未来可独立）
│       │   └── src/lib.rs
│       └── telemetry/            # 流式输出模型
│           └── src/lib.rs
│
├── sdk/                          # 多语言 SDK（未来）
│   ├── go/
│   ├── typescript/
│   └── python/
│
├── deploy/                       # 部署配置
│   ├── docker/
│   ├── k8s/
│   └── compose/
│
├── docs/
│   ├── architecture/
│   ├── plans/
│   └── api/
│
├── examples/
│   └── basic-task/
│
├── scripts/                      # 构建、测试脚本
│   ├── build.sh
│   └── proto-gen.sh
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 包边界规则

| 层级 | 可引用 | 不可引用 |
|------|--------|----------|
| cmd/* | internal, pkg, api | 其他 cmd |
| internal/* | internal（同层或下层）, pkg, api | cmd |
| pkg/* | 仅标准库、第三方 | internal, cmd |
| runtime/* | api/proto（通过 tonic-build） | Go 代码 |

---

## 规模预估（10 万行）

| 区域 | 预估行数 | 说明 |
|------|----------|------|
| cmd/ | ~2k | 3 个入口，薄封装 |
| internal/ | ~50k | 核心逻辑，适配器，bootstrap |
| pkg/ | ~3k | 领域模型，配置，事件 |
| runtime/ | ~15k | worker, sandbox, telemetry |
| api/ | ~2k | proto, 生成代码 |
| sdk/ | ~20k | Go/TS/Python SDK |
| deploy/ | ~3k | Docker, K8s, Compose |
| docs/ | ~5k | 架构、API 文档 |
| 测试 | ~20k | 单元、集成、e2e |

---

## 扩展点

### 新增 Skill

```
internal/skills/builtin/<name>.go
internal/adapters/runtimeclient/  # 若需新执行协议
```

### 新增 Planner 模型

```
internal/orchestration/planner/<provider>/
```

### 新增 Messaging 适配器

```
internal/adapters/messaging/<name>/
```

### 新增 Persistence 适配器

```
internal/adapters/persistence/<name>/
```

### 新增 Sandbox 后端

```
runtime/crates/sandbox/src/<provider>.rs
```

---

## 与现有代码的对应

| 现有 | 最终结构 |
|------|----------|
| internal/access/ | 保持 |
| internal/orchestration/ | 保持，未来可拆 planner/ |
| internal/adapters/* | 保持 |
| internal/bootstrap/ | 保持 |
| internal/runtimeclient/ | 保持 |
| pkg/taskdsl/ | 保持 |
| pkg/events/ | 保持 |
| runtime/crates/* | 保持 |
| 无 policy/ | 新增 internal/policy/ |
| 无 skills/ | 新增 internal/skills/ |
| 无 broker/ | 新增 internal/broker/（未来） |
| 无 sdk/ | 新增 sdk/（未来） |

---

## 迁移建议

1. **Phase 1**：保持现有结构，仅补充文档
2. **Phase 2**：实现 internal/policy/、internal/skills/
3. **Phase 3**：实现 internal/broker/，支持多 Worker
4. **Phase 4**：新增 sdk/go、sdk/typescript

无需一次性重构，按需逐步对齐即可。
