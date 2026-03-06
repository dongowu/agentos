package worker

import (
	"context"

	adapterruntime "github.com/dongowu/agentos/internal/adapters/runtimeclient"
	"github.com/dongowu/agentos/internal/runtimeclient"
)

// GRPCDialer dials worker runtime services over gRPC.
type GRPCDialer struct{}

// NewGRPCDialer returns a dialer backed by the runtime gRPC client adapter.
func NewGRPCDialer() *GRPCDialer {
	return &GRPCDialer{}
}

// Dial connects to a worker runtime address.
func (d *GRPCDialer) Dial(ctx context.Context, addr string) (runtimeclient.ExecutorClient, error) {
	return adapterruntime.NewGRPCExecutorClient(ctx, addr)
}

var _ Dialer = (*GRPCDialer)(nil)
