package orchestration

import (
	"context"
	"testing"
)

func TestPromptPlanner_UsesOriginalPromptAsCommand(t *testing.T) {
	planner := &PromptPlanner{}
	plan, err := planner.Plan(context.Background(), PlanInput{TaskID: "task-1", Prompt: "echo hello"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	cmd, _ := plan.Actions[0].Payload["cmd"].(string)
	if cmd != "echo hello" {
		t.Fatalf("expected command echo hello, got %q", cmd)
	}
}

func TestPromptPlanner_RecognizesFileRead(t *testing.T) {
	planner := &PromptPlanner{}
	plan, err := planner.Plan(context.Background(), PlanInput{TaskID: "task-2", Prompt: "read /tmp/demo.txt"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	if plan.Actions[0].Kind != "file.read" {
		t.Fatalf("expected file.read, got %q", plan.Actions[0].Kind)
	}
	path, _ := plan.Actions[0].Payload["path"].(string)
	if path != "/tmp/demo.txt" {
		t.Fatalf("expected path /tmp/demo.txt, got %q", path)
	}
}

func TestPromptPlanner_RecognizesFileWrite(t *testing.T) {
	planner := &PromptPlanner{}
	plan, err := planner.Plan(context.Background(), PlanInput{TaskID: "task-3", Prompt: "write hello world to /tmp/out.txt"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	if plan.Actions[0].Kind != "file.write" {
		t.Fatalf("expected file.write, got %q", plan.Actions[0].Kind)
	}
	if got, _ := plan.Actions[0].Payload["content"].(string); got != "hello world" {
		t.Fatalf("expected content hello world, got %q", got)
	}
	if got, _ := plan.Actions[0].Payload["path"].(string); got != "/tmp/out.txt" {
		t.Fatalf("expected path /tmp/out.txt, got %q", got)
	}
}

func TestPromptPlanner_RecognizesHTTPRequest(t *testing.T) {
	planner := &PromptPlanner{}
	plan, err := planner.Plan(context.Background(), PlanInput{TaskID: "task-4", Prompt: "fetch https://example.com/data.json"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	if plan.Actions[0].Kind != "http.request" {
		t.Fatalf("expected http.request, got %q", plan.Actions[0].Kind)
	}
	if got, _ := plan.Actions[0].Payload["url"].(string); got != "https://example.com/data.json" {
		t.Fatalf("expected url, got %q", got)
	}
	if got, _ := plan.Actions[0].Payload["method"].(string); got != "GET" {
		t.Fatalf("expected GET, got %q", got)
	}
}

func TestPromptPlanner_SplitsThenIntoMultipleActions(t *testing.T) {
	planner := &PromptPlanner{}
	plan, err := planner.Plan(context.Background(), PlanInput{TaskID: "task-5", Prompt: "read /tmp/in.txt then write copied to /tmp/out.txt"})
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
