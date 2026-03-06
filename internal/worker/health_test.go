package worker

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeEventBus captures published events for test assertions.
type fakeEventBus struct {
	mu     sync.Mutex
	events []fakeEvent
}

type fakeEvent struct {
	topic   string
	payload any
}

func (b *fakeEventBus) Publish(_ context.Context, topic string, payload any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, fakeEvent{topic: topic, payload: payload})
	return nil
}

func (b *fakeEventBus) Subscribe(string, func(any)) (func(), error) {
	return func() {}, nil
}

func (b *fakeEventBus) getEvents() []fakeEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]fakeEvent, len(b.events))
	copy(cp, b.events)
	return cp
}

func TestHealthMonitor_MarksStaleWorkerOffline(t *testing.T) {
	timeout := 50 * time.Millisecond
	reg := NewMemoryRegistry(timeout)
	bus := &fakeEventBus{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = reg.Register(ctx, WorkerInfo{
		ID:       "w-1",
		Addr:     ":9001",
		MaxTasks: 4,
	})

	mon := NewHealthMonitor(reg, bus, timeout, 20*time.Millisecond)
	go mon.Start(ctx)

	// Wait for the heartbeat to go stale and the monitor to detect it.
	time.Sleep(150 * time.Millisecond)
	cancel()

	workers, _ := reg.List(context.Background())
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].Status != StatusOffline {
		t.Errorf("expected offline, got %s", workers[0].Status)
	}

	evts := bus.getEvents()
	if len(evts) == 0 {
		t.Fatal("expected at least one worker.offline event")
	}
	if evts[0].topic != "worker.offline" {
		t.Errorf("expected topic worker.offline, got %s", evts[0].topic)
	}
}

func TestHealthMonitor_KeepsHealthyWorkerOnline(t *testing.T) {
	timeout := 100 * time.Millisecond
	reg := NewMemoryRegistry(timeout)
	bus := &fakeEventBus{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = reg.Register(ctx, WorkerInfo{
		ID:       "w-1",
		Addr:     ":9001",
		MaxTasks: 4,
	})

	mon := NewHealthMonitor(reg, bus, timeout, 30*time.Millisecond)
	go mon.Start(ctx)

	// Keep heartbeating so the worker stays alive.
	for i := 0; i < 4; i++ {
		time.Sleep(25 * time.Millisecond)
		_ = reg.Heartbeat(ctx, "w-1", 0)
	}

	cancel()

	workers, _ := reg.List(context.Background())
	if workers[0].Status != StatusOnline {
		t.Errorf("expected online, got %s", workers[0].Status)
	}

	evts := bus.getEvents()
	if len(evts) != 0 {
		t.Errorf("expected no offline events, got %d", len(evts))
	}
}

func TestHealthMonitor_StopsOnContextCancel(t *testing.T) {
	reg := NewMemoryRegistry(time.Second)
	bus := &fakeEventBus{}
	ctx, cancel := context.WithCancel(context.Background())

	mon := NewHealthMonitor(reg, bus, time.Second, 50*time.Millisecond)

	done := make(chan struct{})
	go func() {
		mon.Start(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(time.Second):
		t.Fatal("HealthMonitor.Start did not return after context cancel")
	}
}
