package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/dongowu/agentos/api/gen/agentos/v1"
	"github.com/dongowu/agentos/internal/bootstrap"
	"github.com/dongowu/agentos/internal/worker"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := bootstrap.FromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()

	// Resolve gRPC listen address (default :50052).
	listenAddr := os.Getenv("GRPC_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":50052"
	}

	// Boot gRPC server for WorkerRegistry.
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", listenAddr, err)
	}

	grpcServer := grpc.NewServer()
	regServer := worker.NewRegistryServer(app.WorkerRegistry)
	pb.RegisterWorkerRegistryServer(grpcServer, regServer)

	go func() {
		log.Printf("WorkerRegistry gRPC server listening on %s", listenAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// Start HealthMonitor if registry is a MemoryRegistry.
	if memReg, ok := app.WorkerRegistry.(*worker.MemoryRegistry); ok {
		hm := worker.NewHealthMonitor(memReg, app.Bus, 30*time.Second, 10*time.Second)
		go hm.Start(ctx)
		log.Println("HealthMonitor started")
	}

	log.Println("controller started")

	// Wait for SIGINT or SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
	cancel()
	grpcServer.GracefulStop()
	log.Println("controller stopped")
}
