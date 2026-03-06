package worker

import (
	"context"
	"testing"

	pb "github.com/dongowu/agentos/api/gen/agentos/v1"
)

func TestRemoteRegistry_ListsAndFiltersWorkers(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()
	_, _ = client.Register(ctx, &pb.RegisterRequest{WorkerId: "w-1", Addr: "127.0.0.1:9001", MaxTasks: 2})
	_, _ = client.Register(ctx, &pb.RegisterRequest{WorkerId: "w-2", Addr: "127.0.0.1:9002", MaxTasks: 2})
	_, _ = client.Heartbeat(ctx, &pb.HeartbeatRequest{WorkerId: "w-1", ActiveTasks: 2})

	remote := NewRemoteRegistryClient(client)
	workers, err := remote.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(workers))
	}

	available, err := remote.GetAvailable(ctx)
	if err != nil {
		t.Fatalf("GetAvailable: %v", err)
	}
	if len(available) != 1 || available[0].ID != "w-2" {
		t.Fatalf("expected only w-2 available, got %+v", available)
	}
}
