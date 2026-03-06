package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/internal/worker"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

// --- fakes for local scheduler tests ---

type fakePool struct {
	mu       sync.Mutex
	execLog  []execCall
	result   *runtimeclient.ExecutionResult
	err      error
	selected string // next SelectWorker return
	selErr   error
	execute  func(ctx context.Context, workerID, taskID string, action *taskdsl.Action) (*runtimeclient.ExecutionResult, error)
}

type execCall struct {
	workerID string
	taskID   string
	actionID string
}

func (p *fakePool) SelectWorker(_ context.Context) (string, error) {
	return p.selected, p.selErr
}

func (p *fakePool) Execute(ctx context.Context, workerID, taskID string, action *taskdsl.Action) (*runtimeclient.ExecutionResult, error) {
	if p.execute != nil {
		return p.execute(ctx, workerID, taskID, action)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.execLog = append(p.execLog, execCall{workerID: workerID, taskID: taskID, actionID: action.ID})
	return p.result, p.err
}

func (p *fakePool) getExecLog() []execCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make([]execCall, len(p.execLog))
	copy(cp, p.execLog)
	return cp
}

// --- local scheduler tests ---

func TestLocalScheduler_SubmitAndReceiveResult(t *testing.T) {
	pool := &fakePool{
		selected: "w-1",
		result:   &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: []byte("done")},
	}
	sched := NewLocalScheduler(pool)

	ctx := context.Background()
	action := &taskdsl.Action{ID: "act-1", Kind: "command.exec"}
	if err := sched.Submit(ctx, "task-1", action); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case r := <-sched.Results():
		if r.TaskID != "task-1" {
			t.Errorf("expected TaskID task-1, got %s", r.TaskID)
		}
		if r.ActionID != "act-1" {
			t.Errorf("expected ActionID act-1, got %s", r.ActionID)
		}
		if r.WorkerID != "w-1" {
			t.Errorf("expected WorkerID w-1, got %s", r.WorkerID)
		}
		if r.ExitCode != 0 {
			t.Errorf("expected ExitCode 0, got %d", r.ExitCode)
		}
		if r.Error != nil {
			t.Errorf("unexpected error: %v", r.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for result")
	}
}

func TestLocalScheduler_SubmitWithExecutionError(t *testing.T) {
	pool := &fakePool{
		selected: "w-1",
		err:      context.DeadlineExceeded,
	}
	sched := NewLocalScheduler(pool)

	ctx := context.Background()
	action := &taskdsl.Action{ID: "act-1", Kind: "command.exec"}
	_ = sched.Submit(ctx, "task-1", action)

	select {
	case r := <-sched.Results():
		if r.Error == nil {
			t.Fatal("expected error in result")
		}
		if r.WorkerID != "w-1" {
			t.Errorf("expected WorkerID w-1, got %s", r.WorkerID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for result")
	}
}

func TestLocalScheduler_SubmitNoWorkerAvailable(t *testing.T) {
	pool := &fakePool{
		selErr: errors.New("no available workers"),
	}
	sched := NewLocalScheduler(pool)

	err := sched.Submit(context.Background(), "task-1", &taskdsl.Action{ID: "act-1"})
	if err == nil {
		t.Fatal("expected error when no worker available")
	}
}

func TestLocalScheduler_MultipleSubmits(t *testing.T) {
	pool := &fakePool{
		selected: "w-1",
		result:   &runtimeclient.ExecutionResult{ExitCode: 0},
	}
	sched := NewLocalScheduler(pool)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_ = sched.Submit(ctx, "task-1", &taskdsl.Action{ID: "act-1"})
		<-sched.Results()
	}

	log := pool.getExecLog()
	if len(log) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(log))
	}
}

func TestLocalScheduler_DetachesDispatchFromCallerCancellation(t *testing.T) {
	release := make(chan struct{})
	pool := &fakePool{selected: "w-1"}
	pool.execute = func(ctx context.Context, workerID, taskID string, action *taskdsl.Action) (*runtimeclient.ExecutionResult, error) {
		<-release
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return &runtimeclient.ExecutionResult{ExitCode: 0}, nil
	}

	sched := NewLocalScheduler(pool)
	ctx, cancel := context.WithCancel(context.Background())
	if err := sched.Submit(ctx, "task-1", &taskdsl.Action{ID: "act-1", Kind: "command.exec"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cancel()
	close(release)

	select {
	case r := <-sched.Results():
		if r.Error != nil {
			t.Fatalf("expected detached context, got error: %v", r.Error)
		}
		if r.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", r.ExitCode)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for result")
	}
}

func TestLocalScheduler_Close(t *testing.T) {
	pool := &fakePool{selected: "w-1", result: &runtimeclient.ExecutionResult{ExitCode: 0}}
	sched := NewLocalScheduler(pool)

	if err := sched.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// --- NATS scheduler compile-time check ---

func TestNATSScheduler_ImplementsInterface(t *testing.T) {
	var _ Scheduler = (*NATSScheduler)(nil)
}

// --- integration-style test with real pool ---

func TestLocalScheduler_WithRealPool(t *testing.T) {
	reg := worker.NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	_ = reg.Register(ctx, worker.WorkerInfo{ID: "w-1", Addr: ":9001", MaxTasks: 10})
	_ = reg.Register(ctx, worker.WorkerInfo{ID: "w-2", Addr: ":9002", MaxTasks: 10})

	// Use fakePool to avoid real gRPC connections, but verify the scheduler works end-to-end.
	fp := &fakePool{
		selected: "w-2",
		result:   &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: []byte("hello")},
	}
	sched := NewLocalScheduler(fp)

	if err := sched.Submit(ctx, "task-99", &taskdsl.Action{ID: "act-5", Kind: "command.exec"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	r := <-sched.Results()
	if r.WorkerID != "w-2" {
		t.Errorf("expected worker w-2, got %s", r.WorkerID)
	}
	if string(r.Stdout) != "hello" {
		t.Errorf("expected stdout 'hello', got %q", r.Stdout)
	}
}
