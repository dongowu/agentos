package worker

import (
	"context"
	"log"

	pb "github.com/dongowu/agentos/api/gen/agentos/v1"
)

// RegistryServer implements the WorkerRegistry gRPC service by delegating
// to the Registry interface.
type RegistryServer struct {
	pb.UnimplementedWorkerRegistryServer
	registry Registry
}

// NewRegistryServer creates a gRPC server backed by the given registry.
func NewRegistryServer(reg Registry) *RegistryServer {
	return &RegistryServer{registry: reg}
}

// Register handles a worker registration request.
func (s *RegistryServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	info := WorkerInfo{
		ID:           req.GetWorkerId(),
		Addr:         req.GetAddr(),
		Capabilities: req.GetCapabilities(),
		MaxTasks:     int(req.GetMaxTasks()),
	}

	if err := s.registry.Register(ctx, info); err != nil {
		log.Printf("[grpc] register failed worker=%s: %v", req.GetWorkerId(), err)
		return &pb.RegisterResponse{Accepted: false}, nil
	}

	log.Printf("[grpc] registered worker=%s addr=%s", req.GetWorkerId(), req.GetAddr())
	return &pb.RegisterResponse{Accepted: true}, nil
}

// Heartbeat handles a periodic heartbeat from a worker.
func (s *RegistryServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if err := s.registry.Heartbeat(ctx, req.GetWorkerId(), int(req.GetActiveTasks())); err != nil {
		log.Printf("[grpc] heartbeat failed worker=%s: %v", req.GetWorkerId(), err)
		return &pb.HeartbeatResponse{Ok: false}, nil
	}

	return &pb.HeartbeatResponse{Ok: true}, nil
}

// Deregister handles a worker deregistration request.
func (s *RegistryServer) Deregister(ctx context.Context, req *pb.DeregisterRequest) (*pb.DeregisterResponse, error) {
	if err := s.registry.Deregister(ctx, req.GetWorkerId()); err != nil {
		log.Printf("[grpc] deregister failed worker=%s: %v", req.GetWorkerId(), err)
		return &pb.DeregisterResponse{Ok: false}, nil
	}

	log.Printf("[grpc] deregistered worker=%s", req.GetWorkerId())
	return &pb.DeregisterResponse{Ok: true}, nil
}
