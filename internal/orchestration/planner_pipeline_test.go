package orchestration

import (
	"context"
	"errors"
	"testing"

	"github.com/dongowu/agentos/internal/adapters/llm"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

type stubPlanner struct {
	calls int
	plan  *taskdsl.Plan
	err   error
	plans []*taskdsl.Plan
	errs  []error
}

func (p *stubPlanner) Plan(_ context.Context, _ PlanInput) (*taskdsl.Plan, error) {
	p.calls++
	if len(p.errs) >= p.calls && p.errs[p.calls-1] != nil {
		return nil, p.errs[p.calls-1]
	}
	if len(p.plans) >= p.calls && p.plans[p.calls-1] != nil {
		return p.plans[p.calls-1], nil
	}
	if p.err != nil {
		return nil, p.err
	}
	return p.plan, nil
}

func TestRetryPlanner_RetriesThenSucceeds(t *testing.T) {
	inner := &stubPlanner{
		errs: []error{context.DeadlineExceeded, nil},
		plans: []*taskdsl.Plan{nil, {
			Actions: []taskdsl.Action{{ID: "a1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo ok"}}},
		}},
	}
	planner := NewRetryPlanner(inner, 2)

	plan, err := planner.Plan(context.Background(), PlanInput{Prompt: "echo ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inner.calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", inner.calls)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != "command.exec" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestRetryPlanner_DoesNotRetryMalformedPlan(t *testing.T) {
	inner := &stubPlanner{err: ErrMalformedPlan}
	planner := NewRetryPlanner(inner, 3)

	_, err := planner.Plan(context.Background(), PlanInput{Prompt: "echo hi"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrMalformedPlan) {
		t.Fatalf("expected ErrMalformedPlan, got %v", err)
	}
	if inner.calls != 1 {
		t.Fatalf("expected malformed plan to stop retries, got %d calls", inner.calls)
	}
}

func TestFallbackPlanner_UsesSecondaryOnPrimaryError(t *testing.T) {
	primary := &stubPlanner{err: errors.New("provider down")}
	secondary := &stubPlanner{plan: &taskdsl.Plan{Actions: []taskdsl.Action{{ID: "b1", Kind: "file.read", RuntimeEnv: "default", Payload: map[string]any{"path": "/tmp/a"}}}}}
	planner := NewFallbackPlanner(primary, secondary)

	plan, err := planner.Plan(context.Background(), PlanInput{Prompt: "read /tmp/a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if primary.calls != 1 || secondary.calls != 1 {
		t.Fatalf("expected both planners to be used once, got primary=%d secondary=%d", primary.calls, secondary.calls)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != "file.read" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestPromptPlanner_SplitsSemicolonsAndNewlines(t *testing.T) {
	planner := &PromptPlanner{}

	plan, err := planner.Plan(context.Background(), PlanInput{Prompt: "read /tmp/in.txt;\nwrite done to /tmp/out.txt"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(plan.Actions))
	}
	if plan.Actions[0].Kind != "file.read" {
		t.Fatalf("expected first action file.read, got %q", plan.Actions[0].Kind)
	}
	if plan.Actions[1].Kind != "file.write" {
		t.Fatalf("expected second action file.write, got %q", plan.Actions[1].Kind)
	}
}

func TestFallbackPlanner_UsesPromptPlannerWhenPrimaryReturnsMalformedPlanError(t *testing.T) {
	provider := &mockProvider{response: &llm.Response{Content: "not valid plan text"}}
	primary := NewLLMPlanner(provider, "gpt-4")
	secondary := &PromptPlanner{}
	planner := NewFallbackPlanner(primary, secondary)

	plan, err := planner.Plan(context.Background(), PlanInput{Prompt: "read /tmp/a then write done to /tmp/b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Actions) != 2 {
		t.Fatalf("expected prompt planner fallback with 2 actions, got %#v", plan)
	}
	if plan.Actions[0].Kind != "file.read" || plan.Actions[1].Kind != "file.write" {
		t.Fatalf("unexpected fallback plan: %#v", plan)
	}
}

func TestFallbackPlanner_UsesPromptPlannerWhenRepairFails(t *testing.T) {
	provider := &mockProvider{responses: []*llm.Response{{Content: "not json"}, {Content: "still bad"}}}
	primary := NewLLMPlanner(provider, "gpt-4")
	secondary := &PromptPlanner{}
	planner := NewFallbackPlanner(primary, secondary)

	plan, err := planner.Plan(context.Background(), PlanInput{Prompt: "read /tmp/a then write done to /tmp/b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Actions) != 2 {
		t.Fatalf("expected prompt planner fallback with 2 actions, got %#v", plan)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("expected primary planner to attempt repair, got %d provider calls", len(provider.requests))
	}
}
