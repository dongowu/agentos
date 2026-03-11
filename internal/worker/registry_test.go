package worker

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRegistry_RegisterAndList(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	w := WorkerInfo{
		ID:           "w-1",
		Addr:         "localhost:9001",
		Capabilities: []string{"native", "docker"},
		MaxTasks:     4,
	}

	if err := reg.Register(ctx, w); err != nil {
		t.Fatalf("Register: %v", err)
	}

	workers, err := reg.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].ID != "w-1" {
		t.Errorf("expected ID w-1, got %s", workers[0].ID)
	}
	if workers[0].Status != StatusOnline {
		t.Errorf("expected status online, got %s", workers[0].Status)
	}
}

func TestMemoryRegistry_RegisterDuplicateRefreshesWorkerSnapshot(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	w := WorkerInfo{ID: "w-1", Addr: "localhost:9001", MaxTasks: 4}

	if err := reg.Register(ctx, w); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	before, err := reg.List(ctx)
	if err != nil {
		t.Fatalf("List before refresh: %v", err)
	}
	firstBeat := before[0].LastHeartbeat
	time.Sleep(5 * time.Millisecond)

	updated := WorkerInfo{ID: "w-1", Addr: "localhost:9002", MaxTasks: 8, Capabilities: []string{"shell"}}
	if err := reg.Register(ctx, updated); err != nil {
		t.Fatalf("second Register: %v", err)
	}

	workers, err := reg.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].Addr != "localhost:9002" {
		t.Fatalf("expected address to refresh, got %q", workers[0].Addr)
	}
	if workers[0].MaxTasks != 8 {
		t.Fatalf("expected max tasks 8, got %d", workers[0].MaxTasks)
	}
	if workers[0].Status != StatusOnline {
		t.Fatalf("expected worker to be online after re-register, got %s", workers[0].Status)
	}
	if !workers[0].LastHeartbeat.After(firstBeat) {
		t.Fatal("expected re-register to refresh LastHeartbeat")
	}
}

func TestMemoryRegistry_Deregister(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	w := WorkerInfo{ID: "w-1", Addr: "localhost:9001", MaxTasks: 4}
	_ = reg.Register(ctx, w)

	if err := reg.Deregister(ctx, "w-1"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	workers, _ := reg.List(ctx)
	if len(workers) != 0 {
		t.Fatalf("expected 0 workers after deregister, got %d", len(workers))
	}
}

func TestMemoryRegistry_DeregisterNotFound(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	if err := reg.Deregister(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error on deregister of unknown worker")
	}
}

func TestMemoryRegistry_Heartbeat(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	w := WorkerInfo{ID: "w-1", Addr: "localhost:9001", MaxTasks: 4}
	_ = reg.Register(ctx, w)

	before, _ := reg.List(ctx)
	firstBeat := before[0].LastHeartbeat

	// Small sleep so time advances.
	time.Sleep(5 * time.Millisecond)

	if err := reg.Heartbeat(ctx, "w-1", 2); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}

	after, _ := reg.List(ctx)
	if !after[0].LastHeartbeat.After(firstBeat) {
		t.Error("heartbeat did not advance LastHeartbeat")
	}
	if after[0].ActiveTasks != 2 {
		t.Errorf("expected ActiveTasks=2, got %d", after[0].ActiveTasks)
	}
}

func TestMemoryRegistry_HeartbeatNotFound(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	if err := reg.Heartbeat(ctx, "nonexistent", 0); err == nil {
		t.Fatal("expected error on heartbeat for unknown worker")
	}
}

func TestMemoryRegistry_GetAvailable(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	_ = reg.Register(ctx, WorkerInfo{ID: "w-1", Addr: ":9001", MaxTasks: 2})
	_ = reg.Register(ctx, WorkerInfo{ID: "w-2", Addr: ":9002", MaxTasks: 2})

	// w-1 is at capacity.
	_ = reg.Heartbeat(ctx, "w-1", 2)

	avail, err := reg.GetAvailable(ctx)
	if err != nil {
		t.Fatalf("GetAvailable: %v", err)
	}
	if len(avail) != 1 {
		t.Fatalf("expected 1 available worker, got %d", len(avail))
	}
	if avail[0].ID != "w-2" {
		t.Errorf("expected w-2 to be available, got %s", avail[0].ID)
	}
}

func TestMemoryRegistry_GetAvailable_ExcludesOffline(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	_ = reg.Register(ctx, WorkerInfo{ID: "w-1", Addr: ":9001", MaxTasks: 4})

	// Force the worker offline via MarkOffline.
	reg.MarkOffline("w-1")

	avail, _ := reg.GetAvailable(ctx)
	if len(avail) != 0 {
		t.Fatalf("expected 0 available (offline), got %d", len(avail))
	}
}

func TestMemoryRegistry_HeartbeatStatusTransition(t *testing.T) {
	reg := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	_ = reg.Register(ctx, WorkerInfo{ID: "w-1", Addr: ":9001", MaxTasks: 2})

	// At capacity -> busy.
	_ = reg.Heartbeat(ctx, "w-1", 2)
	workers, _ := reg.List(ctx)
	if workers[0].Status != StatusBusy {
		t.Errorf("expected busy when at capacity, got %s", workers[0].Status)
	}

	// Below capacity -> online.
	_ = reg.Heartbeat(ctx, "w-1", 1)
	workers, _ = reg.List(ctx)
	if workers[0].Status != StatusOnline {
		t.Errorf("expected online when below capacity, got %s", workers[0].Status)
	}
}
