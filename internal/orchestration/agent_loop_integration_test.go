package orchestration

import (
	"context"
	"testing"

	"github.com/dongowu/agentos/internal/adapters/llm"
	msgmemory "github.com/dongowu/agentos/internal/adapters/messaging/memory"
	persmemory "github.com/dongowu/agentos/internal/adapters/persistence/memory"
	"github.com/dongowu/agentos/internal/tool"
)

func TestAgentLoop_Integration_MultiStepTask(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	// Simulate a 3-step agent interaction:
	// 1. LLM calls shell("echo step1")
	// 2. LLM calls shell("echo step2")
	// 3. LLM returns final answer
	provider := &mockLLMProvider{
		responses: []llm.Response{
			{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "shell", Arguments: `{"cmd":"echo step1"}`}}},
			{ToolCalls: []llm.ToolCall{{ID: "c2", Name: "shell", Arguments: `{"cmd":"echo step2"}`}}},
			{Content: "Both steps completed successfully"},
		},
	}

	shellTool := &mockTool{
		name:   "shell",
		desc:   "Execute commands",
		result: map[string]any{"stdout": "ok", "exit_code": 0},
	}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")
	engine.WithTools([]tool.Tool{shellTool})

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "run two steps")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	if task.State != string(Succeeded) {
		t.Errorf("expected succeeded, got %s", task.State)
	}
	if task.Result != "Both steps completed successfully" {
		t.Errorf("unexpected result: %q", task.Result)
	}
	if provider.calls != 3 {
		t.Errorf("expected 3 LLM calls, got %d", provider.calls)
	}
	if shellTool.calls != 2 {
		t.Errorf("expected 2 tool calls, got %d", shellTool.calls)
	}

	// Verify conversation history: last request should have all prior messages
	lastReq := provider.requests[2]
	// system + user + assistant(tool_call) + tool(result) + assistant(tool_call) + tool(result) = 6
	if len(lastReq.Messages) != 6 {
		t.Errorf("expected 6 messages in final request, got %d", len(lastReq.Messages))
	}
}

func TestAgentLoop_Integration_ToolError_ContinuesLoop(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	provider := &mockLLMProvider{
		responses: []llm.Response{
			{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "nonexistent", Arguments: `{}`}}},
			{Content: "Tool not found, but I can still answer"},
		},
	}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "try bad tool")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Errorf("expected succeeded, got %s", task.State)
	}
	if task.Result != "Tool not found, but I can still answer" {
		t.Errorf("unexpected result: %q", task.Result)
	}
}

func TestAgentLoop_Integration_MultipleToolCallsPerIteration(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	// LLM calls two tools in one iteration, then returns answer
	provider := &mockLLMProvider{
		responses: []llm.Response{
			{ToolCalls: []llm.ToolCall{
				{ID: "c1", Name: "shell", Arguments: `{"cmd":"ls"}`},
				{ID: "c2", Name: "shell", Arguments: `{"cmd":"pwd"}`},
			}},
			{Content: "Listed files and showed directory"},
		},
	}

	shellTool := &mockTool{
		name:   "shell",
		desc:   "Execute commands",
		result: map[string]any{"stdout": "output", "exit_code": 0},
	}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")
	engine.WithTools([]tool.Tool{shellTool})

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "list files and show dir")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Errorf("expected succeeded, got %s", task.State)
	}
	if shellTool.calls != 2 {
		t.Errorf("expected 2 tool calls in single iteration, got %d", shellTool.calls)
	}
	if provider.calls != 2 {
		t.Errorf("expected 2 LLM calls, got %d", provider.calls)
	}
}
