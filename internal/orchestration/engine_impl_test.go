package orchestration

import (
	"context"
	"testing"

	msgmemory "github.com/dongowu/ai-orchestrator/internal/adapters/messaging/memory"
	persmemory "github.com/dongowu/ai-orchestrator/internal/adapters/persistence/memory"
)

func TestEngineImpl_StartTask_RunsPlanningAndTransitionsToQueued(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	skillResolver := &StubSkillResolver{}
	engine := NewEngineImpl(repo, bus, planner, skillResolver, nil)
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	if task.State != string(Queued) {
		t.Errorf("expected state queued, got %s", task.State)
	}
	if task.Plan == nil {
		t.Fatal("expected plan attached")
	}
	if len(task.Plan.Actions) == 0 {
		t.Fatal("expected at least one action")
	}
}
