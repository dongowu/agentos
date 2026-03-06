package orchestration

import (
	"context"
	"strings"
	"testing"

	msgmemory "github.com/dongowu/agentos/internal/adapters/messaging/memory"
	persmemory "github.com/dongowu/agentos/internal/adapters/persistence/memory"
	"github.com/dongowu/agentos/internal/memory"
	"github.com/dongowu/agentos/internal/policy"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

type recordingPlanner struct {
	lastInput PlanInput
	plan      *taskdsl.Plan
}

func (p *recordingPlanner) Plan(_ context.Context, input PlanInput) (*taskdsl.Plan, error) {
	p.lastInput = input
	if p.plan != nil {
		return p.plan, nil
	}
	return &taskdsl.Plan{Actions: []taskdsl.Action{{
		ID:         "action-1",
		Kind:       "command.exec",
		RuntimeEnv: "default",
		Payload:    map[string]any{"cmd": "echo hello"},
	}}}, nil
}

type recordingExecutor struct {
	lastAction *taskdsl.Action
	result     *runtimeclient.ExecutionResult
	called     int
}

func (e *recordingExecutor) ExecuteAction(_ context.Context, _ string, action *taskdsl.Action) (*runtimeclient.ExecutionResult, error) {
	e.called++
	clone := *action
	e.lastAction = &clone
	if e.result != nil {
		return e.result, nil
	}
	return &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: []byte("ok")}, nil
}

type recordingMemoryProvider struct {
	searchCalled bool
	putCalled    bool
	putKey       string
	putValue     []byte
}

func (m *recordingMemoryProvider) Put(_ context.Context, key string, value []byte) error {
	m.putCalled = true
	m.putKey = key
	m.putValue = append([]byte(nil), value...)
	return nil
}

func (m *recordingMemoryProvider) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}

func (m *recordingMemoryProvider) Search(_ context.Context, _ string, _ int) ([]memory.SearchResult, error) {
	m.searchCalled = true
	return []memory.SearchResult{{Key: "task:prev", Content: []byte("previous successful deploy"), Score: 0.9}}, nil
}

func TestEngineImpl_StartTaskWithInput_PassesAgentNameToPolicy(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &recordingPlanner{}
	pol := &mockPolicyEngine{decision: &policy.PolicyDecision{Allowed: true}}
	exec := &recordingExecutor{}
	engine := NewEngineImpl(repo, bus, planner, nil, exec, pol, nil)

	_, err := engine.StartTaskWithInput(context.Background(), StartTaskInput{
		Prompt:    "echo hello",
		AgentName: "agent-1",
		TenantID:  "tenant-1",
	})
	if err != nil {
		t.Fatalf("StartTaskWithInput: %v", err)
	}
	if pol.lastReq.AgentName != "agent-1" {
		t.Fatalf("expected agent-1, got %q", pol.lastReq.AgentName)
	}
	if pol.lastReq.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1, got %q", pol.lastReq.TenantID)
	}
}

func TestEngineImpl_StartTaskWithInput_UsesMemoryRecallAndStoresResult(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &recordingPlanner{}
	exec := &recordingExecutor{result: &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: []byte("done")}}
	mem := &recordingMemoryProvider{}
	engine := NewEngineImpl(repo, bus, planner, nil, exec, nil, nil).WithMemoryHook(NewMemoryHook(mem))

	_, err := engine.StartTaskWithInput(context.Background(), StartTaskInput{Prompt: "deploy project"})
	if err != nil {
		t.Fatalf("StartTaskWithInput: %v", err)
	}
	if !mem.searchCalled {
		t.Fatal("expected memory recall before planning")
	}
	if !strings.Contains(planner.lastInput.Prompt, "previous successful deploy") {
		t.Fatalf("expected planner prompt to include recalled context, got %q", planner.lastInput.Prompt)
	}
	if !mem.putCalled || !strings.HasPrefix(mem.putKey, "task:") {
		t.Fatalf("expected result stored in memory, key=%q called=%v", mem.putKey, mem.putCalled)
	}
}

func TestEngineImpl_StartTaskWithInput_InjectsCredentialToken(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &recordingPlanner{plan: &taskdsl.Plan{Actions: []taskdsl.Action{{
		ID:         "action-1",
		Kind:       "command.exec",
		RuntimeEnv: "default",
		Payload:    map[string]any{"cmd": "echo token", "env": map[string]any{"FOO": "bar"}},
	}}}}
	exec := &recordingExecutor{}
	vault := policy.NewInMemoryVault(map[string]string{"agent-1": "super-secret"})
	engine := NewEngineImpl(repo, bus, planner, nil, exec, nil, nil).WithVault(vault)

	_, err := engine.StartTaskWithInput(context.Background(), StartTaskInput{Prompt: "echo token", AgentName: "agent-1"})
	if err != nil {
		t.Fatalf("StartTaskWithInput: %v", err)
	}
	if exec.lastAction == nil {
		t.Fatal("expected action passed to executor")
	}
	env, ok := exec.lastAction.Payload["env"].(map[string]any)
	if !ok {
		t.Fatalf("expected env map in payload, got %#v", exec.lastAction.Payload["env"])
	}
	token, ok := env["AGENTOS_CREDENTIAL_TOKEN"].(string)
	if !ok || token == "" {
		t.Fatalf("expected injected credential token, got %#v", env["AGENTOS_CREDENTIAL_TOKEN"])
	}
	if token == "super-secret" {
		t.Fatal("expected opaque token, not raw secret")
	}
}
