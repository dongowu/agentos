package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dongowu/agentos/internal/adapters/llm"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

// mockProvider is a test double for llm.Provider.
type mockProvider struct {
	response  *llm.Response
	err       error
	responses []*llm.Response
	errs      []error
	requests  []llm.Request
}

func (m *mockProvider) Chat(_ context.Context, req llm.Request) (*llm.Response, error) {
	m.requests = append(m.requests, req)
	index := len(m.requests) - 1
	if len(m.errs) > index && m.errs[index] != nil {
		return nil, m.errs[index]
	}
	if len(m.responses) > index && m.responses[index] != nil {
		return m.responses[index], nil
	}
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

func TestLLMPlanner_Plan_InvalidJSON_ReturnsMalformedPlanError(t *testing.T) {
	provider := &mockProvider{
		response: &llm.Response{Content: "I don't understand the request"},
	}
	planner := NewLLMPlanner(provider, "gpt-4")

	_, err := planner.Plan(context.Background(), PlanInput{
		TaskID: "task-3",
		Prompt: "do something",
	})
	if err == nil {
		t.Fatal("expected malformed plan error")
	}
	if !errors.Is(err, ErrMalformedPlan) {
		t.Fatalf("expected ErrMalformedPlan, got %v", err)
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

func TestLLMPlanner_Plan_EmptyActions_ReturnsMalformedPlanError(t *testing.T) {
	plan := taskdsl.Plan{Actions: []taskdsl.Action{}}
	planJSON, _ := json.Marshal(plan)

	provider := &mockProvider{
		response: &llm.Response{Content: string(planJSON)},
	}
	planner := NewLLMPlanner(provider, "gpt-4")

	_, err := planner.Plan(context.Background(), PlanInput{
		TaskID: "task-5",
		Prompt: "nothing",
	})
	if err == nil {
		t.Fatal("expected malformed plan error")
	}
	if !errors.Is(err, ErrMalformedPlan) {
		t.Fatalf("expected ErrMalformedPlan, got %v", err)
	}
}

func TestLLMPlanner_Plan_ExtractsJSONFromMixedContent(t *testing.T) {
	provider := &mockProvider{
		response: &llm.Response{Content: "I will create a plan now.\n```json\n{\"Actions\":[{\"Kind\":\"command.exec\",\"Payload\":{\"cmd\":\"echo hello\"}}]}\n```"},
	}
	planner := NewLLMPlanner(provider, "gpt-4")

	result, err := planner.Plan(context.Background(), PlanInput{TaskID: "task-mixed", Prompt: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].ID == "" {
		t.Fatal("expected normalized action id")
	}
	if result.Actions[0].RuntimeEnv == "" {
		t.Fatal("expected normalized runtime env")
	}
}

func TestLLMPlanner_Plan_AcceptsBareActionArray(t *testing.T) {
	provider := &mockProvider{
		response: &llm.Response{Content: `[{"Kind":"command.exec","Payload":{"cmd":"echo one"}},{"Kind":"file.read","Payload":{"path":"/tmp/in.txt"}}]`},
	}
	planner := NewLLMPlanner(provider, "gpt-4")

	result, err := planner.Plan(context.Background(), PlanInput{TaskID: "task-array", Prompt: "echo one then read /tmp/in.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(result.Actions))
	}
	if result.Actions[1].Kind != "file.read" {
		t.Fatalf("expected file.read, got %q", result.Actions[1].Kind)
	}
}

func TestLLMPlanner_Plan_RepairsMalformedOutput(t *testing.T) {
	provider := &mockProvider{responses: []*llm.Response{
		{Content: "here is the plan: nope"},
		{Content: `{"Actions":[{"Kind":"command.exec","Payload":{"cmd":"echo repaired"}}]}`},
	}}
	planner := NewLLMPlanner(provider, "gpt-4")

	result, err := planner.Plan(context.Background(), PlanInput{TaskID: "task-repair", Prompt: "echo repaired"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("expected 2 provider calls, got %d", len(provider.requests))
	}
	if len(result.Actions) != 1 || result.Actions[0].Kind != "command.exec" {
		t.Fatalf("unexpected repaired plan: %#v", result)
	}
	last := provider.requests[1]
	if len(last.Messages) == 0 || last.Messages[len(last.Messages)-1].Role != "user" {
		t.Fatalf("expected repair user prompt, got %#v", last.Messages)
	}
	if !strings.Contains(last.Messages[len(last.Messages)-1].Content, "malformed") {
		t.Fatalf("expected repair prompt to mention malformed output, got %q", last.Messages[len(last.Messages)-1].Content)
	}
}

func TestLLMPlanner_Plan_RepairFailureStillReturnsMalformedPlan(t *testing.T) {
	provider := &mockProvider{responses: []*llm.Response{
		{Content: "not json"},
		{Content: "still not json"},
	}}
	planner := NewLLMPlanner(provider, "gpt-4")

	_, err := planner.Plan(context.Background(), PlanInput{TaskID: "task-repair-fail", Prompt: "read /tmp/a then write /tmp/b"})
	if err == nil {
		t.Fatal("expected malformed plan error")
	}
	if !errors.Is(err, ErrMalformedPlan) {
		t.Fatalf("expected ErrMalformedPlan, got %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("expected 2 provider calls, got %d", len(provider.requests))
	}
}
