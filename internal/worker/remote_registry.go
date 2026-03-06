package worker

import (
	"context"
	"fmt"
	"time"

	pb "github.com/dongowu/agentos/api/gen/agentos/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RemoteRegistry implements Registry by delegating to a controller gRPC endpoint.
type RemoteRegistry struct {
	client pb.WorkerRegistryClient
	conn   *grpc.ClientConn
}

// NewRemoteRegistry dials the control-plane WorkerRegistry service.
func NewRemoteRegistry(ctx context.Context, addr string) (*RemoteRegistry, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("remote registry dial: %w", err)
	}
	return &RemoteRegistry{client: pb.NewWorkerRegistryClient(conn), conn: conn}, nil
}

// NewRemoteRegistryClient wraps an already constructed WorkerRegistry client.
func NewRemoteRegistryClient(client pb.WorkerRegistryClient) *RemoteRegistry {
	return &RemoteRegistry{client: client}
}

func (r *RemoteRegistry) Register(ctx context.Context, info WorkerInfo) error {
	resp, err := r.client.Register(ctx, &pb.RegisterRequest{
		WorkerId:     info.ID,
		Addr:         info.Addr,
		Capabilities: append([]string(nil), info.Capabilities...),
		MaxTasks:     int32(info.MaxTasks),
	})
	if err != nil {
		return err
	}
	if !resp.GetAccepted() {
		return fmt.Errorf("register rejected for worker %q", info.ID)
	}
	return nil
}

func (r *RemoteRegistry) Deregister(ctx context.Context, workerID string) error {
	resp, err := r.client.Deregister(ctx, &pb.DeregisterRequest{WorkerId: workerID})
	if err != nil {
		return err
	}
	if !resp.GetOk() {
		return fmt.Errorf("deregister rejected for worker %q", workerID)
	}
	return nil
}

func (r *RemoteRegistry) Heartbeat(ctx context.Context, workerID string, activeTasks int) error {
	resp, err := r.client.Heartbeat(ctx, &pb.HeartbeatRequest{WorkerId: workerID, ActiveTasks: int32(activeTasks)})
	if err != nil {
		return err
	}
	if !resp.GetOk() {
		return fmt.Errorf("heartbeat rejected for worker %q", workerID)
	}
	return nil
}

func (r *RemoteRegistry) List(ctx context.Context) ([]WorkerInfo, error) {
	resp, err := r.client.ListWorkers(ctx, &pb.ListWorkersRequest{})
	if err != nil {
		return nil, err
	}
	return snapshotsToWorkerInfo(resp.GetWorkers()), nil
}

func (r *RemoteRegistry) GetAvailable(ctx context.Context) ([]WorkerInfo, error) {
	resp, err := r.client.ListWorkers(ctx, &pb.ListWorkersRequest{AvailableOnly: true})
	if err != nil {
		return nil, err
	}
	return snapshotsToWorkerInfo(resp.GetWorkers()), nil
}

// Close closes the underlying controller connection when owned by this registry.
func (r *RemoteRegistry) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func snapshotsToWorkerInfo(workers []*pb.WorkerSnapshot) []WorkerInfo {
	out := make([]WorkerInfo, 0, len(workers))
	for _, item := range workers {
		if item == nil {
			continue
		}
		out = append(out, WorkerInfo{
			ID:            item.GetWorkerId(),
			Addr:          item.GetAddr(),
			Capabilities:  append([]string(nil), item.GetCapabilities()...),
			Status:        WorkerStatus(item.GetStatus()),
			LastHeartbeat: time.Unix(item.GetLastHeartbeatUnix(), 0),
			ActiveTasks:   int(item.GetActiveTasks()),
			MaxTasks:      int(item.GetMaxTasks()),
		})
	}
	return out
}

var _ Registry = (*RemoteRegistry)(nil)
