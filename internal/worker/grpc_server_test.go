package worker

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/dongowu/agentos/api/gen/agentos/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// startTestServer boots a gRPC server with a MemoryRegistry using an in-memory listener.
// Returns the client connection and a cleanup function.
func startTestServer(t *testing.T) (pb.WorkerRegistryClient, func()) {
	t.Helper()

	reg := NewMemoryRegistry(30 * time.Second)
	srv := grpc.NewServer()
	pb.RegisterWorkerRegistryServer(srv, NewRegistryServer(reg))

	lis := bufconn.Listen(1024 * 1024)
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatal(err)
	}

	client := pb.NewWorkerRegistryClient(conn)
	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
	return client, cleanup
}

func TestRegister(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	resp, err := client.Register(context.Background(), &pb.RegisterRequest{
		WorkerId:     "w-1",
		Addr:         "127.0.0.1:9000",
		Capabilities: []string{"shell", "docker"},
		MaxTasks:     4,
	})
	if err != nil {
		t.Fatalf("Register RPC failed: %v", err)
	}
	if !resp.Accepted {
		t.Fatal("expected Accepted=true")
	}
}

func TestRegisterDuplicateRefreshesWorkerSnapshot(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()
	req := &pb.RegisterRequest{
		WorkerId: "w-dup",
		Addr:     "127.0.0.1:9001",
		MaxTasks: 2,
	}

	resp, err := client.Register(ctx, req)
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if !resp.Accepted {
		t.Fatal("first register should be accepted")
	}

	updatedReq := &pb.RegisterRequest{
		WorkerId: "w-dup",
		Addr:     "127.0.0.1:9011",
		MaxTasks: 5,
	}
	resp2, err := client.Register(ctx, updatedReq)
	if err != nil {
		t.Fatalf("second Register RPC failed: %v", err)
	}
	if !resp2.Accepted {
		t.Fatal("duplicate register should be accepted as a refresh")
	}

	listResp, err := client.ListWorkers(ctx, &pb.ListWorkersRequest{})
	if err != nil {
		t.Fatalf("ListWorkers failed: %v", err)
	}
	if len(listResp.Workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(listResp.Workers))
	}
	if listResp.Workers[0].GetAddr() != "127.0.0.1:9011" {
		t.Fatalf("expected worker addr to stay refreshed, got %q", listResp.Workers[0].GetAddr())
	}
	if listResp.Workers[0].GetMaxTasks() != 5 {
		t.Fatalf("expected max tasks 5, got %d", listResp.Workers[0].GetMaxTasks())
	}
}

func TestHeartbeat(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()

	_, err := client.Register(ctx, &pb.RegisterRequest{
		WorkerId: "w-hb",
		Addr:     "127.0.0.1:9002",
		MaxTasks: 4,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{
		WorkerId:    "w-hb",
		ActiveTasks: 2,
	})
	if err != nil {
		t.Fatalf("Heartbeat RPC failed: %v", err)
	}
	if !resp.Ok {
		t.Fatal("expected Ok=true")
	}
}

func TestHeartbeatUnknownWorker(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	resp, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		WorkerId:    "ghost",
		ActiveTasks: 0,
	})
	if err != nil {
		t.Fatalf("Heartbeat RPC failed: %v", err)
	}
	if resp.Ok {
		t.Fatal("heartbeat for unknown worker should return Ok=false")
	}
}

func TestDeregister(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()

	_, err := client.Register(ctx, &pb.RegisterRequest{
		WorkerId: "w-dereg",
		Addr:     "127.0.0.1:9003",
		MaxTasks: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Deregister(ctx, &pb.DeregisterRequest{
		WorkerId: "w-dereg",
	})
	if err != nil {
		t.Fatalf("Deregister RPC failed: %v", err)
	}
	if !resp.Ok {
		t.Fatal("expected Ok=true")
	}
}

func TestDeregisterUnknownWorker(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	resp, err := client.Deregister(context.Background(), &pb.DeregisterRequest{
		WorkerId: "ghost",
	})
	if err != nil {
		t.Fatalf("Deregister RPC failed: %v", err)
	}
	if resp.Ok {
		t.Fatal("deregister for unknown worker should return Ok=false")
	}
}

func TestFullLifecycle(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()

	regResp, err := client.Register(ctx, &pb.RegisterRequest{
		WorkerId:     "w-full",
		Addr:         "127.0.0.1:9004",
		Capabilities: []string{"native"},
		MaxTasks:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !regResp.Accepted {
		t.Fatal("register should be accepted")
	}

	hbResp, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{
		WorkerId:    "w-full",
		ActiveTasks: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hbResp.Ok {
		t.Fatal("heartbeat should be ok")
	}

	deregResp, err := client.Deregister(ctx, &pb.DeregisterRequest{
		WorkerId: "w-full",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !deregResp.Ok {
		t.Fatal("deregister should be ok")
	}

	hbResp2, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{
		WorkerId:    "w-full",
		ActiveTasks: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if hbResp2.Ok {
		t.Fatal("heartbeat after deregister should return Ok=false")
	}
}

func TestListWorkers(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()
	_, _ = client.Register(ctx, &pb.RegisterRequest{WorkerId: "w-1", Addr: "127.0.0.1:9001", MaxTasks: 2})
	_, _ = client.Register(ctx, &pb.RegisterRequest{WorkerId: "w-2", Addr: "127.0.0.1:9002", MaxTasks: 2})
	_, _ = client.Heartbeat(ctx, &pb.HeartbeatRequest{WorkerId: "w-1", ActiveTasks: 2})

	resp, err := client.ListWorkers(ctx, &pb.ListWorkersRequest{AvailableOnly: false})
	if err != nil {
		t.Fatalf("ListWorkers RPC failed: %v", err)
	}
	if len(resp.Workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(resp.Workers))
	}

	available, err := client.ListWorkers(ctx, &pb.ListWorkersRequest{AvailableOnly: true})
	if err != nil {
		t.Fatalf("ListWorkers available RPC failed: %v", err)
	}
	if len(available.Workers) != 1 {
		t.Fatalf("expected 1 available worker, got %d", len(available.Workers))
	}
	if available.Workers[0].WorkerId != "w-2" {
		t.Fatalf("expected w-2 to be available, got %s", available.Workers[0].WorkerId)
	}
}
