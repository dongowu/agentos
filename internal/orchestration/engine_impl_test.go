package orchestration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	msgmemory "github.com/dongowu/agentos/internal/adapters/messaging/memory"
	persmemory "github.com/dongowu/agentos/internal/adapters/persistence/memory"
	"github.com/dongowu/agentos/internal/policy"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/internal/scheduler"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

// --- mock policy engine ---

type mockPolicyEngine struct {
	decision *policy.PolicyDecision
	err      error
	called   int
	lastReq  policy.PolicyRequest
}

func (m *mockPolicyEngine) Evaluate(_ context.Context, req policy.PolicyRequest) (*policy.PolicyDecision, error) {
	m.called++
	m.lastReq = req
	return m.decision, m.err
}

// --- mock scheduler ---

type mockScheduler struct {
	submitted []schedulerEntry
	results   chan scheduler.ActionResult
	closed    bool
	err       error
}

type schedulerEntry struct {
	taskID string
	action *taskdsl.Action
}

func newMockScheduler() *mockScheduler {
	return &mockScheduler{results: make(chan scheduler.ActionResult, 16)}
}

func (m *mockScheduler) Submit(_ context.Context, taskID string, action *taskdsl.Action) error {
	if m.err != nil {
		return m.err
	}
	m.submitted = append(m.submitted, schedulerEntry{taskID: taskID, action: action})
	return nil
}

func (m *mockScheduler) Results() <-chan scheduler.ActionResult {
	return m.results
}

func (m *mockScheduler) Close() error {
	m.closed = true
	return nil
}

// --- mock executor ---

type mockExecutor struct {
	result *runtimeclient.ExecutionResult
	err    error
	called int
}

func (m *mockExecutor) ExecuteAction(_ context.Context, _ string, _ *taskdsl.Action) (*runtimeclient.ExecutionResult, error) {
	m.called++
	return m.result, m.err
}

// --- tests ---

func TestEngineImpl_StartTask_RunsPlanningAndTransitionsToQueued(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	skillResolver := &StubSkillResolver{}
	engine := NewEngineImpl(repo, bus, planner, skillResolver, nil, nil, nil)
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

func TestEngineImpl_PolicyDenial_BlocksExecution(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	pol := &mockPolicyEngine{
		decision: &policy.PolicyDecision{Allowed: false, Reason: "tool denied by policy"},
	}
	engine := NewEngineImpl(repo, bus, planner, nil, nil, pol, nil)
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "echo hello")
	if err == nil {
		t.Fatal("expected error from policy denial")
	}
	if !strings.Contains(err.Error(), "policy denied") {
		t.Errorf("expected policy denied error, got: %v", err)
	}
	if pol.called != 1 {
		t.Errorf("expected policy called once, got %d", pol.called)
	}
	// Task should be in failed state
	got, _ := repo.Get(ctx, task.ID)
	if got.State != string(Failed) {
		t.Errorf("expected task state failed, got %s", got.State)
	}
}

func TestEngineImpl_PolicyError_BlocksExecution(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	pol := &mockPolicyEngine{
		err: fmt.Errorf("policy unavailable"),
	}
	engine := NewEngineImpl(repo, bus, planner, nil, nil, pol, nil)
	ctx := context.Background()

	_, err := engine.StartTask(ctx, "echo hello")
	if err == nil {
		t.Fatal("expected error from policy failure")
	}
	if !strings.Contains(err.Error(), "policy") {
		t.Errorf("expected policy error, got: %v", err)
	}
}

func TestEngineImpl_PolicyAllows_ContinuesExecution(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	pol := &mockPolicyEngine{
		decision: &policy.PolicyDecision{Allowed: true, Reason: "allowed"},
	}
	// No executor or scheduler: reaches queued state with dispatched events
	engine := NewEngineImpl(repo, bus, planner, nil, nil, pol, nil)
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Queued) {
		t.Errorf("expected state queued, got %s", task.State)
	}
	if pol.called != 1 {
		t.Errorf("expected policy called once, got %d", pol.called)
	}
}

func TestEngineImpl_SchedulerDispatch_ReturnsRunning(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	sched := newMockScheduler()
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, sched)
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Running) {
		t.Errorf("expected state running, got %s", task.State)
	}
	if len(sched.submitted) != 1 {
		t.Fatalf("expected 1 submitted action, got %d", len(sched.submitted))
	}
	if sched.submitted[0].taskID != task.ID {
		t.Errorf("submitted task ID mismatch")
	}
}

func TestEngineImpl_SchedulerWithPolicy_BothApplied(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	pol := &mockPolicyEngine{
		decision: &policy.PolicyDecision{Allowed: true, Reason: "ok"},
	}
	sched := newMockScheduler()
	engine := NewEngineImpl(repo, bus, planner, nil, nil, pol, sched)
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Running) {
		t.Errorf("expected state running, got %s", task.State)
	}
	if pol.called != 1 {
		t.Errorf("expected policy called once, got %d", pol.called)
	}
	if len(sched.submitted) != 1 {
		t.Errorf("expected 1 submitted action, got %d", len(sched.submitted))
	}
}

func TestEngineImpl_ProcessResults_UpdatesTaskState(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	sched := newMockScheduler()
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, sched)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	// Start background processor
	go engine.ProcessResults(ctx)

	// Send a success result
	sched.results <- scheduler.ActionResult{
		TaskID:   task.ID,
		ActionID: "action-1",
		ExitCode: 0,
	}

	// Wait a bit for the goroutine to process
	time.Sleep(50 * time.Millisecond)

	got, _ := repo.Get(ctx, task.ID)
	if got.State != string(Succeeded) {
		t.Errorf("expected state succeeded, got %s", got.State)
	}
}

func TestEngineImpl_ProcessResults_FailedAction(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	sched := newMockScheduler()
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, sched)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	go engine.ProcessResults(ctx)

	// Send a failure result
	sched.results <- scheduler.ActionResult{
		TaskID:   task.ID,
		ActionID: "action-1",
		ExitCode: 1,
	}

	time.Sleep(50 * time.Millisecond)

	got, _ := repo.Get(ctx, task.ID)
	if got.State != string(Failed) {
		t.Errorf("expected state failed, got %s", got.State)
	}
}

func TestEngineImpl_ProcessResults_NilScheduler_Returns(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)

	// ProcessResults should return immediately when scheduler is nil
	done := make(chan struct{})
	go func() {
		engine.ProcessResults(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ProcessResults did not return for nil scheduler")
	}
}

func TestEngineImpl_SchedulerError_FallsBackToDirectExecutor(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	sched := newMockScheduler()
	sched.err = fmt.Errorf("no available workers")
	exec := &mockExecutor{result: &runtimeclient.ExecutionResult{ExitCode: 0}}
	engine := NewEngineImpl(repo, bus, planner, nil, exec, nil, sched)
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if exec.called != 1 {
		t.Fatalf("expected direct executor called once, got %d", exec.called)
	}
	if task.State != string(Succeeded) {
		t.Fatalf("expected state succeeded, got %s", task.State)
	}
}

func TestEngineImpl_NilPolicyAndScheduler_BackwardCompat(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	// With no executor and no scheduler, should reach queued
	if task.State != string(Queued) {
		t.Errorf("expected state queued, got %s", task.State)
	}
}
