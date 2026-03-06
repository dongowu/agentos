package worker

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/dongowu/agentos/api/gen/agentos/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// startTestServer boots a gRPC server with a MemoryRegistry on a random port.
// Returns the client connection and a cleanup function.
func startTestServer(t *testing.T) (pb.WorkerRegistryClient, func()) {
	t.Helper()

	reg := NewMemoryRegistry(30 * time.Second)
	srv := grpc.NewServer()
	pb.RegisterWorkerRegistryServer(srv, NewRegistryServer(reg))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatal(err)
	}

	client := pb.NewWorkerRegistryClient(conn)
	cleanup := func() {
		conn.Close()
		srv.Stop()
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

func TestRegisterDuplicate(t *testing.T) {
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

	resp2, err := client.Register(ctx, req)
	if err != nil {
		t.Fatalf("second Register RPC failed: %v", err)
	}
	if resp2.Accepted {
		t.Fatal("duplicate register should not be accepted")
	}
}

func TestHeartbeat(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register first.
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

	// Register
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

	// Heartbeat
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

	// Deregister
	deregResp, err := client.Deregister(ctx, &pb.DeregisterRequest{
		WorkerId: "w-full",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !deregResp.Ok {
		t.Fatal("deregister should be ok")
	}

	// Heartbeat after deregister should fail
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
