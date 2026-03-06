package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

// stubExecutor is a fake ExecutorClient for tests.
type stubExecutor struct {
	mu      sync.Mutex
	calls   int
	result  *runtimeclient.ExecutionResult
	err     error
	latency time.Duration
}

func (s *stubExecutor) ExecuteAction(_ context.Context, _ string, _ *taskdsl.Action) (*runtimeclient.ExecutionResult, error) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.result, s.err
}

func (s *stubExecutor) getCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// stubDialer implements Dialer for tests.
type stubDialer struct {
	mu       sync.Mutex
	dialCount int
	clients  map[string]runtimeclient.ExecutorClient
}

func (d *stubDialer) Dial(_ context.Context, addr string) (runtimeclient.ExecutorClient, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dialCount++
	c, ok := d.clients[addr]
	if !ok {
		return nil, errors.New("dial failed: " + addr)
	}
	return c, nil
}

func (d *stubDialer) getDialCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dialCount
}

func TestPool_SelectWorker_LeastLoaded(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	_ = reg.Register(ctx, WorkerInfo{ID: "w-1", Addr: ":9001", MaxTasks: 4})
	_ = reg.Register(ctx, WorkerInfo{ID: "w-2", Addr: ":9002", MaxTasks: 4})

	// w-1 has 3 active tasks, w-2 has 1.
	_ = reg.Heartbeat(ctx, "w-1", 3)
	_ = reg.Heartbeat(ctx, "w-2", 1)

	pool := NewPool(reg, nil)

	id, err := pool.SelectWorker(ctx)
	if err != nil {
		t.Fatalf("SelectWorker: %v", err)
	}
	if id != "w-2" {
		t.Errorf("expected w-2 (least loaded), got %s", id)
	}
}

func TestPool_SelectWorker_NoAvailable(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	pool := NewPool(reg, nil)

	_, err := pool.SelectWorker(context.Background())
	if err == nil {
		t.Fatal("expected error when no workers available")
	}
}

func TestPool_Execute(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	_ = reg.Register(ctx, WorkerInfo{ID: "w-1", Addr: ":9001", MaxTasks: 4})

	exec := &stubExecutor{result: &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: []byte("ok")}}
	dialer := &stubDialer{clients: map[string]runtimeclient.ExecutorClient{":9001": exec}}
	pool := NewPool(reg, dialer)

	action := &taskdsl.Action{ID: "a-1", Kind: "command.exec"}
	result, err := pool.Execute(ctx, "w-1", "task-1", action)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", result.ExitCode)
	}
	if string(result.Stdout) != "ok" {
		t.Errorf("expected stdout 'ok', got %q", result.Stdout)
	}
}

func TestPool_Execute_CachesConnection(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	_ = reg.Register(ctx, WorkerInfo{ID: "w-1", Addr: ":9001", MaxTasks: 4})

	exec := &stubExecutor{result: &runtimeclient.ExecutionResult{ExitCode: 0}}
	dialer := &stubDialer{clients: map[string]runtimeclient.ExecutorClient{":9001": exec}}
	pool := NewPool(reg, dialer)

	action := &taskdsl.Action{ID: "a-1", Kind: "command.exec"}
	_, _ = pool.Execute(ctx, "w-1", "task-1", action)
	_, _ = pool.Execute(ctx, "w-1", "task-1", action)

	if exec.getCalls() != 2 {
		t.Errorf("expected 2 calls, got %d", exec.getCalls())
	}
	if dialer.getDialCount() != 1 {
		t.Errorf("expected dialer called once (cached), got %d", dialer.getDialCount())
	}
	if pool.ClientCount() != 1 {
		t.Errorf("expected 1 cached client, got %d", pool.ClientCount())
	}
}

func TestPool_Execute_WorkerNotFound(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	pool := NewPool(reg, nil)

	action := &taskdsl.Action{ID: "a-1", Kind: "command.exec"}
	_, err := pool.Execute(context.Background(), "nonexistent", "task-1", action)
	if err == nil {
		t.Fatal("expected error for unknown worker")
	}
}

func TestPool_Close(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	_ = reg.Register(ctx, WorkerInfo{ID: "w-1", Addr: ":9001", MaxTasks: 4})

	exec := &stubExecutor{result: &runtimeclient.ExecutionResult{ExitCode: 0}}
	dialer := &stubDialer{clients: map[string]runtimeclient.ExecutorClient{":9001": exec}}
	pool := NewPool(reg, dialer)

	_, _ = pool.Execute(ctx, "w-1", "task-1", &taskdsl.Action{ID: "a-1"})

	if err := pool.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if pool.ClientCount() != 0 {
		t.Errorf("expected 0 clients after close, got %d", pool.ClientCount())
	}
}
