package runtimeclient

import "context"

// RuntimeSpec describes the environment needed for an action.
type RuntimeSpec struct {
	Profile string // golang-dev, rust-dev, sui-move, etc.
}

// RuntimeLease is a handle to an allocated sandbox.
type RuntimeLease struct {
	ID     string
	Spec   RuntimeSpec
	Worker string
}

// RuntimeBroker requests and releases runtime leases.
// Shields the Go control plane from Docker, gVisor, or Firecracker details.
type RuntimeBroker interface {
	Acquire(ctx context.Context, spec RuntimeSpec) (*RuntimeLease, error)
	Release(ctx context.Context, leaseID string) error
}
