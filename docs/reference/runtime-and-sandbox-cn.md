# 运行时与沙箱参考

这份文档说明 AgentOS 当前的运行时平面：动作如何执行、当前沙箱如何工作、以及 worker runtime 现在真正提供了哪些保证。

它描述的是**当前已经实现的能力面**，不是未来态运行时设想。

## 运行时平面概览

运行时平面是 AgentOS 的 Rust 执行侧。

高层上包含：

- `runtime/crates/worker` — gRPC worker 服务
- `runtime/crates/sandbox` — runtime adapter 与安全校验
- `runtime/crates/telemetry` — 流式输出模型
- `internal/adapters/runtimeclient` — Go 控制面的 gRPC 客户端
- `internal/worker` — Go 侧的 registry、pool 与 worker 选择逻辑

## 当前执行模型

一个控制面 action 到达运行时平面的路径大致如下：

1. Go 控制面先选择 worker
2. 通过 gRPC 向 `RuntimeService` 发请求
3. Rust worker 把 action payload 转成 `ExecutionSpec`
4. `ActionExecutor` 在执行前应用 `SecurityPolicy`
5. 选定的 runtime adapter 执行命令
6. stdout / stderr 必要时会被截断，并在返回前做 secret redaction
7. worker 返回一次性结果，或者按 chunk 流式返回

## gRPC 运行时接口面

worker 当前暴露两个 RPC：

| RPC | 用途 |
|-----|------|
| `ExecuteAction` | 执行并一次性返回 stdout / stderr / exit code |
| `StreamOutput` | 按流返回 stdout / stderr chunk，并在末尾返回结束结果 |

Proto 摘要：

```proto
service RuntimeService {
  rpc ExecuteAction(ExecuteActionRequest) returns (ExecuteActionResponse);
  rpc StreamOutput(StreamOutputRequest) returns (stream StreamChunk);
}
```

重要字段包括：

- `task_id`
- `action_id`
- `payload` / 运行时负载字节
- 流式 `StreamChunk.kind`，例如 `stdout`、`stderr`、`resource`

## 执行契约

### `ExecutionSpec`

每个 runtime 都执行一个 `ExecutionSpec`，主要字段包括：

| 字段 | 含义 |
|------|------|
| `command` | shell 命令字符串 |
| `working_dir` | 可选工作目录 |
| `env` | 在隔离后额外注入的环境变量 |
| `timeout` | 墙钟超时 |
| `max_output_bytes` | 输出截断上限 |

### `ExecutionResult`

每个完成的 action 会返回：

| 字段 | 含义 |
|------|------|
| `exit_code` | 进程退出码 |
| `stdout` | 捕获的 stdout 字节 |
| `stderr` | 捕获的 stderr 字节 |
| `duration` | 执行耗时 |
| `truncated` | 输出是否发生截断 |

## Runtime Adapters

### Native runtime

当前特征：

- runtime 名称：`native`
- 命令直接在宿主机 OS 上执行
- 启动时会探测可用 shell
- 在非 Windows 系统上优先尝试 `sh`，然后 `bash`
- 在 Windows 上可回退到 `bash`、`sh`、`cmd` 和 `COMSPEC`
- 支持通过 `current_dir(...)` 切换工作目录
- 执行前会清空进程环境，再回注一小组安全变量和调用方 env

当前安全环境变量透传列表：

- `PATH`
- `HOME`
- `TERM`
- `LANG`
- `USER`
- `SHELL`
- `TMPDIR`

从运行角度看，native runtime 意味着：

- 有 shell 访问能力
- 有宿主机文件系统访问能力
- 是最快的本地执行路径
- 隔离性弱于容器执行路径

### Docker runtime

当前特征：

- runtime 名称：`docker`
- 通过构造 `docker run` 命令执行
- 默认使用 `--rm` 和 `--init`
- 支持显式网络模式
- 支持内存与 CPU 限制
- 可开启 `--read-only`
- 可选地把宿主 workspace 挂载到容器内的 `/workspace`
- 通过 `--env` 透传安全环境变量和调用方 env

当前命令层特征：

- 配置了网络时会加 `--network <mode>`
- 配置了内存时会加 `--memory <n>m`
- 配置了 CPU 时会加 `--cpus <limit>`
- 开启只读时会加 `--read-only`
- 开启 workspace 挂载时会加 `--volume <host>:/workspace:rw` 和 `--workdir /workspace`
- 最终执行形式为 `image + sh -c <command>`

当前挂载安全检查：

- 开启挂载时，`ExecutionSpec` 必须带 `working_dir`
- 工作目录必须能解析成绝对路径
- 明确拒绝挂载 `/`
- 如果配置了 `allowed_workspace_roots`，挂载路径必须位于允许目录内

基于 worker 默认环境变量的 Docker 默认姿态：

- image：`ubuntu:22.04`
- network：`none`
- memory：`512 MB`
- CPU：`1.0`
- workspace mount：关闭
- read-only rootfs：关闭

## 安全策略

Rust worker 会在执行前用 `SecurityPolicy` 做命令校验。

### Autonomy 级别

| 级别 | 行为 |
|------|------|
| `supervised` | 只有白名单命令允许执行；未知命令会被拒绝 |
| `semi` / `semi_autonomous` | 仍然是白名单主导，但语义上位于 supervised 和 autonomous 之间 |
| `autonomous` | 只要不命中显式 deny 规则就允许执行 |

### 默认白名单示例

默认白名单包含一些基础命令，例如：

- `ls`
- `cat`
- `echo`
- `pwd`
- `head`
- `tail`
- `grep`
- `find`
- `wc`

### 默认黑名单示例

默认黑名单包含一些危险模式，例如：

- `rm -rf /`
- `rm -rf /*`
- `mkfs.*`
- `dd if=/dev/*`
- `chmod 777 *`
- `:(){ :|:& };:`

### 禁止路径检查

默认策略会阻止命令引用敏感路径，例如：

- `/etc/shadow`
- `/etc/passwd`

### 输出与频率限制

默认限制包括：

- `max_actions_per_hour = 120`
- `max_output_bytes = 1_048_576`

## Secret Redaction

输出离开 worker 之前，安全层会对疑似密钥进行脱敏。

当前会尝试匹配的模式包括：

- API key，例如 `sk-...`
- Bearer Token
- AWS access key
- GitHub token，例如 `ghp_...`
- 泛化的 `key=...`、`token=...`、`secret=...`、`password=...` 形式

脱敏后的占位内容为：

```text
[REDACTED]
```

## 输出截断

一次性执行路径和流式路径都会执行输出上限控制。

当前行为：

- 输出会被截断到 `max_output_bytes`
- 截断时会尽量保持 UTF-8 边界
- 截断后会追加标记

当前截断标记：

```text
... [output truncated]
```

在 streaming 模式下，一旦达到限制，worker 会发送一个截断 chunk，并停止继续转发后续数据。

## 流式输出行为

当控制面使用 streaming 执行路径时：

- native 和 docker runtime 都支持 stdout / stderr 流式输出
- chunk 会带 `kind = stdout` 或 `kind = stderr`
- secret redaction 会在 chunk 发出前执行
- streaming 与 one-shot 共用同一套输出预算控制
- timeout 会被映射成 gRPC deadline 风格错误

代表性 chunk 结构：

```json
{
  "task_id": "task-123",
  "action_id": "act-1",
  "kind": "stdout",
  "data": "aGVsbG8="
}
```

## Worker 注册与心跳

当设置了 `AGENTOS_CONTROL_PLANE_ADDR` 时，worker runtime 还会参与控制面注册链路。

当前行为：

- worker 先启动自己的 gRPC 服务
- 然后向 controller 的 `WorkerRegistry` 注册
- 上报 worker id、监听地址、capabilities 和 max task count
- 按配置间隔启动心跳循环
- 如果没有设置 control-plane 地址，会跳过注册，但仍然可以本地提供执行服务

## 当前稳定运行时面

今天可以把运行时平面的稳定能力理解为：

- `native` runtime
- `docker` runtime
- `SecurityPolicy` 命令校验
- secret redaction
- 输出截断
- 一次性 execute RPC
- 流式 output RPC
- 对共享 controller registry 的 worker 注册与心跳

## 当前还不是稳定运行时面的内容

这些内容在历史或概念文档里出现过，但**不属于**当前稳定实现：

- gVisor runtime
- Firecracker runtime
- WASM runtime
- Rust worker 内部的 browser-specialized runtime
- 完整控制台级实时终端字节流会话

## 兼容层与弃用项

`sandbox` crate 里仍保留了一套兼容层：

- `SandboxSpec`
- `SandboxHandle`
- `IsolationProvider`
- 旧的 `WorkerService`

当前代码应优先使用：

- `ExecutionSpec`
- `ExecutionResult`
- `RuntimeAdapter`
- `ActionExecutor`

## 下一步阅读

- [配置参考](configuration-cn.md)
- [API 能力面参考](api-surfaces-cn.md)
- [核心能力参考](core-capabilities-cn.md)
- [架构概览](../architecture/overview.md)
