package orchestration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/actionbridge"
	"github.com/dongowu/agentos/internal/adapters/llm"
	msgmemory "github.com/dongowu/agentos/internal/adapters/messaging/memory"
	persmemory "github.com/dongowu/agentos/internal/adapters/persistence/memory"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/internal/policy"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/internal/scheduler"
	"github.com/dongowu/agentos/internal/tool"
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
	result    *runtimeclient.ExecutionResult
	err       error
	called    int
	actionIDs []string
}

func (m *mockExecutor) ExecuteAction(_ context.Context, _ string, action *taskdsl.Action) (*runtimeclient.ExecutionResult, error) {
	m.called++
	if action != nil {
		m.actionIDs = append(m.actionIDs, action.ID)
	}
	return m.result, m.err
}

type sequencePlanner struct {
	plan *taskdsl.Plan
	err  error
}

func (p sequencePlanner) Plan(_ context.Context, _ PlanInput) (*taskdsl.Plan, error) {
	return p.plan, p.err
}

type mockSkillResolver struct {
	resolve func(action *taskdsl.Action) (string, error)
}

func (m mockSkillResolver) Resolve(action *taskdsl.Action) (string, error) {
	if m.resolve != nil {
		return m.resolve(action)
	}
	return "default", nil
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

func TestEngineImpl_DirectExecution_RunsMultiStepPlanInOrder(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &recordingPlanner{plan: &taskdsl.Plan{Actions: []taskdsl.Action{
		{ID: "action-1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo first"}},
		{ID: "action-2", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo second"}},
	}}}
	exec := &mockExecutor{result: &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: []byte("ok")}}
	engine := NewEngineImpl(repo, bus, planner, nil, exec, nil, nil)

	task, err := engine.StartTask(context.Background(), "echo first then echo second")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Fatalf("expected state succeeded, got %s", task.State)
	}
	if exec.called != 2 {
		t.Fatalf("expected executor called twice, got %d", exec.called)
	}
	if len(exec.actionIDs) != 2 || exec.actionIDs[0] != "action-1" || exec.actionIDs[1] != "action-2" {
		t.Fatalf("expected action order [action-1 action-2], got %v", exec.actionIDs)
	}
}

func TestEngineImpl_ProcessResults_FailsOnSecondActionAndAppendsAudit(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &recordingPlanner{plan: &taskdsl.Plan{Actions: []taskdsl.Action{
		{ID: "action-1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo first"}},
		{ID: "action-2", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo second"}},
	}}}
	sched := newMockScheduler()
	audit := &mockAuditStore{}
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, sched).WithAuditStore(audit)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task, err := engine.StartTask(ctx, "echo first then echo second")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	go engine.ProcessResults(ctx)

	sched.results <- scheduler.ActionResult{TaskID: task.ID, ActionID: "action-1", ExitCode: 0, Stdout: []byte("first")}
	for deadline := time.Now().Add(300 * time.Millisecond); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if len(sched.submitted) >= 2 {
			break
		}
	}
	if len(sched.submitted) != 2 {
		t.Fatalf("expected second action submission, got %d", len(sched.submitted))
	}

	sched.results <- scheduler.ActionResult{TaskID: task.ID, ActionID: "action-2", ExitCode: 1, Stderr: []byte("boom")}
	var got *taskdsl.Task
	for deadline := time.Now().Add(300 * time.Millisecond); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		got, _ = repo.Get(ctx, task.ID)
		if got != nil && got.State == string(Failed) {
			break
		}
	}
	if got == nil || got.State != string(Failed) {
		t.Fatalf("expected state failed after second action, got %#v", got)
	}
	if len(audit.records) != 2 {
		t.Fatalf("expected 2 audit records, got %d", len(audit.records))
	}
	if audit.records[0].ActionID != "action-1" || audit.records[1].ActionID != "action-2" {
		t.Fatalf("unexpected audit records: %+v", audit.records)
	}
}

func TestEngineImpl_ProcessResults_SubmitsNextActionBeforeCompletingTask(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &recordingPlanner{plan: &taskdsl.Plan{Actions: []taskdsl.Action{
		{ID: "action-1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo first"}},
		{ID: "action-2", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo second"}},
	}}}
	sched := newMockScheduler()
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, sched)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task, err := engine.StartTask(ctx, "echo first then echo second")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if len(sched.submitted) != 1 || sched.submitted[0].action.ID != "action-1" {
		t.Fatalf("expected first submitted action action-1, got %+v", sched.submitted)
	}

	go engine.ProcessResults(ctx)
	sched.results <- scheduler.ActionResult{TaskID: task.ID, ActionID: "action-1", ExitCode: 0}

	for deadline := time.Now().Add(300 * time.Millisecond); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if len(sched.submitted) >= 2 {
			break
		}
	}
	if len(sched.submitted) != 2 {
		t.Fatalf("expected second action to be submitted, got %d submissions", len(sched.submitted))
	}
	if sched.submitted[1].action.ID != "action-2" {
		t.Fatalf("expected second submitted action action-2, got %s", sched.submitted[1].action.ID)
	}
	got, _ := repo.Get(ctx, task.ID)
	if got.State == string(Succeeded) {
		t.Fatalf("expected task not yet succeeded after first result")
	}

	sched.results <- scheduler.ActionResult{TaskID: task.ID, ActionID: "action-2", ExitCode: 0}
	for deadline := time.Now().Add(300 * time.Millisecond); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		got, _ = repo.Get(ctx, task.ID)
		if got.State == string(Succeeded) {
			break
		}
	}
	if got.State != string(Succeeded) {
		t.Fatalf("expected state succeeded after second result, got %s", got.State)
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

type mockAuditStore struct {
	records []persistence.AuditRecord
}

func (m *mockAuditStore) Append(_ context.Context, record persistence.AuditRecord) error {
	m.records = append(m.records, record)
	return nil
}

func (m *mockAuditStore) Get(_ context.Context, taskID, actionID string) (*persistence.AuditRecord, error) {
	for i := range m.records {
		if m.records[i].TaskID == taskID && m.records[i].ActionID == actionID {
			record := m.records[i]
			return &record, nil
		}
	}
	return nil, nil
}

func (m *mockAuditStore) ListByTask(_ context.Context, taskID string) ([]persistence.AuditRecord, error) {
	var out []persistence.AuditRecord
	for _, record := range m.records {
		if record.TaskID == taskID {
			out = append(out, record)
		}
	}
	return out, nil
}

func (m *mockAuditStore) Query(_ context.Context, query persistence.AuditQuery) ([]persistence.AuditRecord, error) {
	var out []persistence.AuditRecord
	for _, record := range m.records {
		if query.TaskID != "" && record.TaskID != query.TaskID {
			continue
		}
		out = append(out, record)
	}
	return out, nil
}

func TestEngineImpl_ProcessResults_AppendsAuditRecord(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	sched := newMockScheduler()
	audit := &mockAuditStore{}
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, sched).WithAuditStore(audit)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	go engine.ProcessResults(ctx)
	sched.results <- scheduler.ActionResult{
		TaskID:   task.ID,
		ActionID: "action-1",
		ExitCode: 0,
		Stdout:   []byte("done"),
		Stderr:   []byte("warn"),
		WorkerID: "worker-1",
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if len(audit.records) == 1 {
			break
		}
	}
	if len(audit.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(audit.records))
	}
	record := audit.records[0]
	if record.TaskID != task.ID || record.ActionID != "action-1" {
		t.Fatalf("unexpected record identity: %+v", record)
	}
	if record.Command != "echo ok" {
		t.Fatalf("expected command echo ok, got %q", record.Command)
	}
	if record.WorkerID != "worker-1" {
		t.Fatalf("expected worker worker-1, got %q", record.WorkerID)
	}
	if record.Stdout != "done" || record.Stderr != "warn" {
		t.Fatalf("unexpected audit outputs: %+v", record)
	}
}

func TestEngineImpl_ProcessResults_AppendsAuditOwnership(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	sched := newMockScheduler()
	audit := &mockAuditStore{}
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, sched).WithAuditStore(audit)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task, err := engine.StartTaskWithInput(ctx, StartTaskInput{Prompt: "echo hello", TenantID: "tenant-a", AgentName: "ops-agent"})
	if err != nil {
		t.Fatalf("StartTaskWithInput: %v", err)
	}

	go engine.ProcessResults(ctx)
	sched.results <- scheduler.ActionResult{TaskID: task.ID, ActionID: "action-1", ExitCode: 0}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if len(audit.records) == 1 {
			break
		}
	}
	if len(audit.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(audit.records))
	}
	if audit.records[0].TenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %q", audit.records[0].TenantID)
	}
	if audit.records[0].AgentName != "ops-agent" {
		t.Fatalf("expected ops-agent, got %q", audit.records[0].AgentName)
	}
}

func TestEngineImpl_DirectExecution_AppendsAuditRecord(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	exec := &mockExecutor{result: &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: []byte("done"), Stderr: []byte("warn")}}
	audit := &mockAuditStore{}
	engine := NewEngineImpl(repo, bus, planner, nil, exec, nil, nil).WithAuditStore(audit)
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Fatalf("expected state succeeded, got %s", task.State)
	}
	if len(audit.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(audit.records))
	}
	record := audit.records[0]
	if record.Command != "echo ok" {
		t.Fatalf("expected command echo ok, got %q", record.Command)
	}
	if record.Stdout != "done" || record.Stderr != "warn" {
		t.Fatalf("unexpected audit outputs: %+v", record)
	}
}

// --- mock LLM provider for agent loop ---

type mockLLMProvider struct {
	responses []llm.Response
	calls     int
	requests  []llm.Request
}

func (m *mockLLMProvider) Chat(_ context.Context, req llm.Request) (*llm.Response, error) {
	m.requests = append(m.requests, req)
	idx := m.calls
	m.calls++
	if idx >= len(m.responses) {
		return &llm.Response{Content: "done"}, nil
	}
	return &m.responses[idx], nil
}

// --- mock tool for agent loop ---

type mockTool struct {
	name   string
	desc   string
	result any
	err    error
	calls  int
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.desc }
func (m *mockTool) Run(_ context.Context, _ map[string]any) (any, error) {
	m.calls++
	return m.result, m.err
}
func (m *mockTool) Schema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"cmd": map[string]any{"type": "string"}}}
}

func TestEngineImpl_AgentLoop_SingleToolCall(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	provider := &mockLLMProvider{
		responses: []llm.Response{
			{ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "shell", Arguments: `{"cmd":"echo hi"}`}}},
			{Content: "The command output: hi"},
		},
	}

	shellTool := &mockTool{name: "shell", desc: "Execute commands", result: map[string]any{"stdout": "hi", "exit_code": 0}}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")
	engine.WithTools([]tool.Tool{shellTool})

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "run echo hi")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Errorf("expected succeeded, got %s", task.State)
	}
	if task.Result != "The command output: hi" {
		t.Errorf("expected result 'The command output: hi', got %q", task.Result)
	}
	if provider.calls != 2 {
		t.Errorf("expected 2 LLM calls, got %d", provider.calls)
	}
	if shellTool.calls != 1 {
		t.Errorf("expected 1 tool call, got %d", shellTool.calls)
	}
}

func TestEngineImpl_AgentLoop_MaxIterations(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	infiniteToolCall := llm.Response{
		ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "shell", Arguments: `{"cmd":"echo loop"}`}},
	}
	responses := make([]llm.Response, 20)
	for i := range responses {
		responses[i] = infiniteToolCall
	}
	provider := &mockLLMProvider{responses: responses}

	shellTool := &mockTool{name: "shell", desc: "Execute commands", result: map[string]any{"stdout": "loop", "exit_code": 0}}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")
	engine.WithTools([]tool.Tool{shellTool})

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "infinite loop")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Failed) {
		t.Errorf("expected failed, got %s", task.State)
	}
	if provider.calls != 10 {
		t.Errorf("expected exactly 10 LLM calls (max iterations), got %d", provider.calls)
	}
}

func TestEngineImpl_AgentLoop_NoToolCalls_ImmediateAnswer(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	provider := &mockLLMProvider{
		responses: []llm.Response{
			{Content: "The answer is 42"},
		},
	}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "what is the answer")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Errorf("expected succeeded, got %s", task.State)
	}
	if task.Result != "The answer is 42" {
		t.Errorf("expected 'The answer is 42', got %q", task.Result)
	}
	if provider.calls != 1 {
		t.Errorf("expected 1 LLM call, got %d", provider.calls)
	}
}

func TestEngineImpl_AgentLoop_FallbackWithoutProvider(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Queued) {
		t.Errorf("expected queued (legacy path), got %s", task.State)
	}
}

func TestEngineImpl_SkillResolverError_FailsTask(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}
	resolver := mockSkillResolver{resolve: func(action *taskdsl.Action) (string, error) {
		return "", fmt.Errorf("unsupported action kind: %s", action.Kind)
	}}
	engine := NewEngineImpl(repo, bus, planner, resolver, nil, nil, nil)
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "echo hello")
	if err == nil {
		t.Fatal("expected resolver error")
	}
	if task == nil {
		t.Fatal("expected task")
	}
	got, getErr := repo.Get(ctx, task.ID)
	if getErr != nil {
		t.Fatalf("repo.Get: %v", getErr)
	}
	if got.State != string(Failed) {
		t.Fatalf("expected failed state, got %s", got.State)
	}
}

func TestEngineImpl_ProcessResults_SkillResolverError_FailsFollowupAction(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &sequencePlanner{plan: &taskdsl.Plan{Actions: []taskdsl.Action{
		{ID: "action-1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo first"}},
		{ID: "action-2", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo second"}},
	}}}
	resolver := mockSkillResolver{resolve: func(action *taskdsl.Action) (string, error) {
		if action.ID == "action-2" {
			return "", fmt.Errorf("unsupported action kind: %s", action.Kind)
		}
		return "default", nil
	}}
	sched := newMockScheduler()
	engine := NewEngineImpl(repo, bus, planner, resolver, nil, nil, sched)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	go engine.ProcessResults(ctx)
	sched.results <- scheduler.ActionResult{TaskID: task.ID, ActionID: "action-1", ExitCode: 0}

	var got *taskdsl.Task
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		got, _ = repo.Get(ctx, task.ID)
		if got != nil && got.State == string(Failed) {
			break
		}
	}
	if got == nil {
		t.Fatal("expected task from repo")
	}
	if got.State != string(Failed) {
		t.Fatalf("expected failed state after follow-up resolution error, got %s", got.State)
	}
}

func TestEngineImpl_DirectExecution_UsesActionBridgeWithoutWorker(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	path := filepath.Join(t.TempDir(), "bridge-direct.txt")
	planner := sequencePlanner{plan: &taskdsl.Plan{Actions: []taskdsl.Action{{ID: "action-1", Kind: "file.write", Payload: map[string]any{"path": path, "content": "from-bridge"}}}}}
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil).WithActionBridge(actionbridge.New())
	ctx := context.Background()

	task, err := engine.StartTask(ctx, "write file")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Fatalf("expected succeeded, got %s", task.State)
	}
}
