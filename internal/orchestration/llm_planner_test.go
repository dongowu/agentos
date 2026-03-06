package orchestration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dongowu/agentos/internal/adapters/llm"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

// mockProvider is a test double for llm.Provider.
type mockProvider struct {
	response *llm.Response
	err      error
}

func (m *mockProvider) Chat(_ context.Context, _ llm.Request) (*llm.Response, error) {
	return m.response, m.err
}

func TestLLMPlanner_Plan_ValidJSON(t *testing.T) {
	plan := taskdsl.Plan{
		Actions: []taskdsl.Action{
			{ID: "a1", Kind: "command.exec", RuntimeEnv: "golang-dev", Payload: map[string]any{"cmd": "go test ./..."}},
			{ID: "a2", Kind: "file.write", RuntimeEnv: "default", Payload: map[string]any{"path": "/tmp/out.txt", "content": "done"}},
		},
	}
	planJSON, _ := json.Marshal(plan)

	provider := &mockProvider{
		response: &llm.Response{Content: string(planJSON)},
	}
	planner := NewLLMPlanner(provider, "gpt-4")

	result, err := planner.Plan(context.Background(), PlanInput{
		TaskID:   "task-1",
		Prompt:   "run tests and write results",
		TenantID: "tenant-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(result.Actions))
	}
	if result.Actions[0].Kind != "command.exec" {
		t.Errorf("expected command.exec, got %s", result.Actions[0].Kind)
	}
	if result.Actions[1].Kind != "file.write" {
		t.Errorf("expected file.write, got %s", result.Actions[1].Kind)
	}
}

func TestLLMPlanner_Plan_JSONInMarkdownBlock(t *testing.T) {
	plan := taskdsl.Plan{
		Actions: []taskdsl.Action{
			{ID: "a1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo hi"}},
		},
	}
	planJSON, _ := json.Marshal(plan)
	// Simulate LLM wrapping JSON in markdown code block.
	wrapped := "```json\n" + string(planJSON) + "\n```"

	provider := &mockProvider{
		response: &llm.Response{Content: wrapped},
	}
	planner := NewLLMPlanner(provider, "gpt-4")

	result, err := planner.Plan(context.Background(), PlanInput{
		TaskID: "task-2",
		Prompt: "echo hi",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result.Actions))
	}
}

func TestLLMPlanner_Plan_InvalidJSON_Fallback(t *testing.T) {
	provider := &mockProvider{
		response: &llm.Response{Content: "I don't understand the request"},
	}
	planner := NewLLMPlanner(provider, "gpt-4")

	result, err := planner.Plan(context.Background(), PlanInput{
		TaskID: "task-3",
		Prompt: "do something",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 fallback action, got %d", len(result.Actions))
	}
	if result.Actions[0].Kind != "command.exec" {
		t.Errorf("expected fallback kind command.exec, got %s", result.Actions[0].Kind)
	}
	cmd, ok := result.Actions[0].Payload["cmd"].(string)
	if !ok || cmd != "do something" {
		t.Errorf("expected fallback cmd 'do something', got %v", result.Actions[0].Payload["cmd"])
	}
}

func TestLLMPlanner_Plan_ProviderError(t *testing.T) {
	provider := &mockProvider{
		err: context.DeadlineExceeded,
	}
	planner := NewLLMPlanner(provider, "gpt-4")

	_, err := planner.Plan(context.Background(), PlanInput{
		TaskID: "task-4",
		Prompt: "test",
	})
	if err == nil {
		t.Fatal("expected error when provider fails")
	}
}

func TestLLMPlanner_Plan_EmptyActions_Fallback(t *testing.T) {
	plan := taskdsl.Plan{Actions: []taskdsl.Action{}}
	planJSON, _ := json.Marshal(plan)

	provider := &mockProvider{
		response: &llm.Response{Content: string(planJSON)},
	}
	planner := NewLLMPlanner(provider, "gpt-4")

	result, err := planner.Plan(context.Background(), PlanInput{
		TaskID: "task-5",
		Prompt: "nothing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty actions should fall back to a single action.
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 fallback action for empty plan, got %d", len(result.Actions))
	}
}
