package scheduler

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/actionbridge"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
	"github.com/nats-io/nats.go"
)

type fakeJetStream struct {
	mu       sync.Mutex
	nextID   int
	handlers map[string]map[int]nats.MsgHandler
	history  []string
}

type fakeNATSSubscription struct {
	once  sync.Once
	unsub func()
}

func newFakeJetStream() *fakeJetStream {
	return &fakeJetStream{handlers: make(map[string]map[int]nats.MsgHandler)}
}

func (f *fakeJetStream) Publish(subject string, data []byte, _ ...nats.PubOpt) (*nats.PubAck, error) {
	f.mu.Lock()
	f.history = append(f.history, subject)
	handlers := make([]nats.MsgHandler, 0, len(f.handlers[subject]))
	for _, handler := range f.handlers[subject] {
		handlers = append(handlers, handler)
	}
	f.mu.Unlock()

	msg := &nats.Msg{Subject: subject, Data: data}
	for _, handler := range handlers {
		handler(msg)
	}
	return &nats.PubAck{}, nil
}

func (f *fakeJetStream) Subscribe(subject string, cb nats.MsgHandler, _ ...nats.SubOpt) (subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := f.nextID
	if f.handlers[subject] == nil {
		f.handlers[subject] = make(map[int]nats.MsgHandler)
	}
	f.handlers[subject][id] = cb
	return &fakeNATSSubscription{unsub: func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.handlers[subject], id)
	}}, nil
}

func (s *fakeNATSSubscription) Unsubscribe() error {
	s.once.Do(func() {
		if s.unsub != nil {
			s.unsub()
		}
	})
	return nil
}

func TestNATSScheduler_DispatchesThroughDispatcher(t *testing.T) {
	js := newFakeJetStream()
	pool := &fakePool{
		selected: "worker-1",
		result:   &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: []byte("done")},
	}

	dispatcher := NewNATSDispatcher(js, "TEST", pool)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer dispatcher.Close()

	sched := NewNATSScheduler(js, "TEST")
	defer sched.Close()

	action := &taskdsl.Action{ID: "act-1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo ok"}}
	if err := sched.Submit(ctx, "task-1", action); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case result := <-sched.Results():
		if result.TaskID != "task-1" {
			t.Fatalf("expected task-1, got %q", result.TaskID)
		}
		if result.WorkerID != "worker-1" {
			t.Fatalf("expected worker-1, got %q", result.WorkerID)
		}
		if string(result.Stdout) != "done" {
			t.Fatalf("expected stdout done, got %q", result.Stdout)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for NATS result")
	}

	if len(pool.getExecLog()) != 1 {
		t.Fatalf("expected exactly one worker execution, got %d", len(pool.getExecLog()))
	}
}

func TestNATSDispatcher_ForwardsStreamingChunks(t *testing.T) {
	js := newFakeJetStream()
	pool := &fakePool{selected: "worker-1"}
	pool.executeStream = func(ctx context.Context, workerID, taskID string, action *taskdsl.Action, sink func(runtimeclient.StreamChunk)) (*runtimeclient.ExecutionResult, error) {
		sink(runtimeclient.StreamChunk{TaskID: taskID, ActionID: action.ID, Kind: "stdout", Data: []byte("na")})
		sink(runtimeclient.StreamChunk{TaskID: taskID, ActionID: action.ID, Kind: "stdout", Data: []byte("ts")})
		return &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: []byte("nats")}, nil
	}

	var streamed []runtimeclient.StreamChunk
	dispatcher := NewNATSDispatcher(js, "TEST", pool).WithOutputHook(func(chunk runtimeclient.StreamChunk) {
		streamed = append(streamed, chunk)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer dispatcher.Close()

	sched := NewNATSScheduler(js, "TEST")
	defer sched.Close()

	action := &taskdsl.Action{ID: "act-2", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo nats"}}
	if err := sched.Submit(ctx, "task-2", action); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case <-sched.Results():
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for NATS result")
	}

	if len(streamed) != 2 {
		t.Fatalf("expected 2 streamed chunks, got %d", len(streamed))
	}
	if string(streamed[0].Data)+string(streamed[1].Data) != "nats" {
		t.Fatalf("unexpected streamed chunks: %+v", streamed)
	}
}

func TestNATSScheduler_UsesConfiguredSubjectPrefix(t *testing.T) {
	js := newFakeJetStream()
	sched := NewNATSScheduler(js, "CUSTOM")
	defer sched.Close()

	if err := sched.Submit(context.Background(), "task-9", &taskdsl.Action{ID: "act-9", Kind: "command.exec"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(js.history) == 0 || js.history[0] != "CUSTOM.actions.dispatch" {
		t.Fatalf("expected first published subject CUSTOM.actions.dispatch, got %v", js.history)
	}
}

func TestNATSDispatcher_BridgeExecutesWithoutWorker(t *testing.T) {
	js := newFakeJetStream()
	pool := &fakePool{selErr: context.DeadlineExceeded}
	dispatcher := NewNATSDispatcher(js, "TEST", pool).WithActionBridge(actionbridge.New())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer dispatcher.Close()

	sched := NewNATSScheduler(js, "TEST")
	defer sched.Close()
	path := filepath.Join(t.TempDir(), "nats-bridge.txt")

	if err := sched.Submit(ctx, "task-bridge", &taskdsl.Action{ID: "act-bridge", Kind: "file.write", Payload: map[string]any{"path": path, "content": "nats bridge"}}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case result := <-sched.Results():
		if result.WorkerID != "control-plane" {
			t.Fatalf("expected control-plane worker id, got %q", result.WorkerID)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", result.ExitCode)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for NATS result")
	}

	if len(pool.getExecLog()) != 0 {
		t.Fatalf("expected no worker execution, got %d", len(pool.getExecLog()))
	}
}
