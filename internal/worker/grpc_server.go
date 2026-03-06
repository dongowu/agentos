package worker

import (
	"context"
	"log"
	"sort"

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

// ListWorkers returns the current worker snapshots for remote schedulers.
func (s *RegistryServer) ListWorkers(ctx context.Context, req *pb.ListWorkersRequest) (*pb.ListWorkersResponse, error) {
	var (
		workers []WorkerInfo
		err     error
	)
	if req.GetAvailableOnly() {
		workers, err = s.registry.GetAvailable(ctx)
	} else {
		workers, err = s.registry.List(ctx)
	}
	if err != nil {
		return nil, err
	}

	sort.Slice(workers, func(i, j int) bool {
		return workers[i].ID < workers[j].ID
	})

	resp := &pb.ListWorkersResponse{Workers: make([]*pb.WorkerSnapshot, 0, len(workers))}
	for _, info := range workers {
		resp.Workers = append(resp.Workers, &pb.WorkerSnapshot{
			WorkerId:          info.ID,
			Addr:              info.Addr,
			Capabilities:      append([]string(nil), info.Capabilities...),
			Status:            string(info.Status),
			ActiveTasks:       int32(info.ActiveTasks),
			MaxTasks:          int32(info.MaxTasks),
			LastHeartbeatUnix: info.LastHeartbeat.Unix(),
		})
	}
	return resp, nil
}
