# Agent Loop Design

**Date**: 2026-03-09
**Status**: Approved
**Scope**: Add tool-calling Agent Loop to EngineImpl

## Problem

Current execution is single-shot: LLM generates a Plan (list of Actions), the engine executes them sequentially, and the task ends. The LLM never sees execution results and cannot adapt, retry, or chain reasoning across steps.

## Solution

Add a tool-calling Agent Loop inside EngineImpl. When an LLM provider with tool-calling support is configured, `StartTaskWithInput` runs a multi-turn loop where the LLM decides which tools to call, observes results, and iterates until the task is complete.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| LLM interaction model | Tool Calling (native) | OpenAI/Claude native format, structured, reliable |
| Max iterations | 10 | Sufficient for simple-to-medium tasks, prevents runaway |
| Implementation location | EngineImpl (modify existing) | Minimal new components, reuses full execution chain |
| Fallback | Existing plan+execute path | No LLM key = current behavior unchanged |

## Module 1: LLM Provider Interface Extension

Extend `internal/adapters/llm/provider.go` with tool-calling support.

### New Types

```go
// ToolDef describes a tool the LLM can invoke.
type ToolDef struct {
    Name        string
    Description string
    Parameters  map[string]any // JSON Schema
}

// ToolCall represents a single tool invocation requested by the LLM.
type ToolCall struct {
    ID        string
    Name      string
    Arguments string // Raw JSON string
}
```

### Extended Types

```go
type Message struct {
    Role       string
    Content    string
    ToolCalls  []ToolCall // assistant messages: tool invocations
    ToolCallID string     // tool messages: correlates to ToolCall.ID
}

type Request struct {
    Model       string
    Messages    []Message
    Temperature float64
    Tools       []ToolDef  // available tools for this request
}

type Response struct {
    Content   string
    ToolCalls []ToolCall // tools the LLM wants to invoke
}
```

**Backward compatibility**: When `Tools` is empty, behavior is identical to current. Existing LLMPlanner is unaffected.

## Module 2: OpenAI Adapter Extension

Extend `internal/adapters/llm/openai/openai.go` to:

1. Send `tools` array in request body when `req.Tools` is non-empty
2. Parse `tool_calls` from response choices
3. Handle `tool` role messages in the messages array

OpenAI tools format:
```json
{
  "tools": [{
    "type": "function",
    "function": {
      "name": "shell",
      "description": "Execute shell commands",
      "parameters": {"type": "object", "properties": {"cmd": {"type": "string"}}}
    }
  }]
}
```

Response parsing: `choices[0].message.tool_calls` array with `id`, `function.name`, `function.arguments`.

## Module 3: EngineImpl Loop Logic

### Flow

```
StartTaskWithInput(ctx, input):
  if LLM provider supports tool-calling:
    → runAgentLoop(ctx, task, input)
  else:
    → existing plan+execute path (unchanged)

runAgentLoop(ctx, task, input):
  1. Build system prompt with available tool descriptions
  2. messages = [system, user(input.Prompt)]
  3. tools = buildToolDefs() from tool.Registry
  4. transition task → planning → running
  5. for i := 0; i < 10; i++:
     a. response = LLM.Chat(messages, tools)
     b. if no tool_calls in response:
        → store response.Content as final answer
        → transition task → evaluating → succeeded
        → return
     c. append assistant message (with tool_calls) to messages
     d. for each tool_call:
        - policy check (deny → fail task)
        - execute tool via tool.Get(name).Run() or executor
        - publish audit event + action output event
        - append tool result message to messages
     e. continue loop
  6. if loop exhausted:
     → transition task → failed (max iterations exceeded)
```

### State Machine

- Task stays in `running` throughout the loop
- Only transitions to `succeeded` or `failed` at loop end
- No changes to state machine transition table needed

### Tool Execution Path

- Built-in tools (shell, file.*, git.*, http.*): call `tool.Get(name).Run(ctx, args)` directly in Go process
- `command.exec` actions: route through existing executor/scheduler to Rust Worker

### Events Published Per Iteration

- `task.action.dispatched` — before each tool execution
- `task.action.output` — stdout/stderr chunks during execution
- `task.action.completed` — after each tool returns
- `task.loop.iteration` — new event: iteration count, tool calls made

## Module 4: Tool Definition Export

Use existing `tool.SchemaAware` interface to extract JSON Schema from tools:

```go
func buildToolDefs(tools []tool.Tool) []llm.ToolDef {
    var defs []llm.ToolDef
    for _, t := range tools {
        def := llm.ToolDef{
            Name:        t.Name(),
            Description: t.Description(),
        }
        if sa, ok := t.(tool.SchemaAware); ok {
            def.Parameters = sa.Schema()
        } else {
            def.Parameters = map[string]any{"type": "object"}
        }
        defs = append(defs, def)
    }
    return defs
}
```

## Task Model Extension

Add field to `pkg/taskdsl/task.go`:

```go
type Task struct {
    ID        string
    Prompt    string
    State     string
    Plan      *Plan
    Result    string    // NEW: final answer from agent loop
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

## Files Changed

| File | Change |
|------|--------|
| `internal/adapters/llm/provider.go` | Add ToolDef, ToolCall, extend Message/Request/Response |
| `internal/adapters/llm/openai/openai.go` | Support tools in request, parse tool_calls in response |
| `internal/orchestration/engine_impl.go` | Add runAgentLoop method, route from StartTaskWithInput |
| `pkg/taskdsl/task.go` | Add Result field |
| `pkg/events/task_events.go` | Add LoopIteration event |
| `internal/orchestration/engine_impl_test.go` | Test agent loop with mock LLM |
| `internal/adapters/llm/openai/openai_test.go` | Test tool_calls request/response |

## What Is NOT Changing

- Existing plan+execute path (no LLM key = same behavior)
- State machine transitions
- Scheduler/Worker/Registry
- Sandbox/SecurityPolicy
- Memory providers
- HTTP API endpoints
- CLI interface
