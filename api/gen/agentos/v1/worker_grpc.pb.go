// Hand-written gRPC service code for the WorkerRegistry service.
// Matches the pattern of runtime_grpc.pb.go and the Rust client
// paths in runtime/crates/worker/src/proto.rs
// (e.g. "/agentos.v1.WorkerRegistry/Register").

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const _ = grpc.SupportPackageIsVersion7

const (
	WorkerRegistry_Register_FullMethodName   = "/agentos.v1.WorkerRegistry/Register"
	WorkerRegistry_Heartbeat_FullMethodName  = "/agentos.v1.WorkerRegistry/Heartbeat"
	WorkerRegistry_Deregister_FullMethodName = "/agentos.v1.WorkerRegistry/Deregister"
)

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

type WorkerRegistryClient interface {
	Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error)
	Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error)
	Deregister(ctx context.Context, in *DeregisterRequest, opts ...grpc.CallOption) (*DeregisterResponse, error)
}

type workerRegistryClient struct {
	cc grpc.ClientConnInterface
}

func NewWorkerRegistryClient(cc grpc.ClientConnInterface) WorkerRegistryClient {
	return &workerRegistryClient{cc}
}

func (c *workerRegistryClient) Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error) {
	out := new(RegisterResponse)
	err := c.cc.Invoke(ctx, WorkerRegistry_Register_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *workerRegistryClient) Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error) {
	out := new(HeartbeatResponse)
	err := c.cc.Invoke(ctx, WorkerRegistry_Heartbeat_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *workerRegistryClient) Deregister(ctx context.Context, in *DeregisterRequest, opts ...grpc.CallOption) (*DeregisterResponse, error) {
	out := new(DeregisterResponse)
	err := c.cc.Invoke(ctx, WorkerRegistry_Deregister_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

type WorkerRegistryServer interface {
	Register(context.Context, *RegisterRequest) (*RegisterResponse, error)
	Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error)
	Deregister(context.Context, *DeregisterRequest) (*DeregisterResponse, error)
	mustEmbedUnimplementedWorkerRegistryServer()
}

type UnimplementedWorkerRegistryServer struct{}

func (UnimplementedWorkerRegistryServer) Register(context.Context, *RegisterRequest) (*RegisterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Register not implemented")
}
func (UnimplementedWorkerRegistryServer) Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Heartbeat not implemented")
}
func (UnimplementedWorkerRegistryServer) Deregister(context.Context, *DeregisterRequest) (*DeregisterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Deregister not implemented")
}
func (UnimplementedWorkerRegistryServer) mustEmbedUnimplementedWorkerRegistryServer() {}

type UnsafeWorkerRegistryServer interface {
	mustEmbedUnimplementedWorkerRegistryServer()
}

func RegisterWorkerRegistryServer(s grpc.ServiceRegistrar, srv WorkerRegistryServer) {
	s.RegisterService(&WorkerRegistry_ServiceDesc, srv)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func _WorkerRegistry_Register_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkerRegistryServer).Register(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: WorkerRegistry_Register_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WorkerRegistryServer).Register(ctx, req.(*RegisterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WorkerRegistry_Heartbeat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HeartbeatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkerRegistryServer).Heartbeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: WorkerRegistry_Heartbeat_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WorkerRegistryServer).Heartbeat(ctx, req.(*HeartbeatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WorkerRegistry_Deregister_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeregisterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkerRegistryServer).Deregister(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: WorkerRegistry_Deregister_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WorkerRegistryServer).Deregister(ctx, req.(*DeregisterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ---------------------------------------------------------------------------
// ServiceDesc
// ---------------------------------------------------------------------------

var WorkerRegistry_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "agentos.v1.WorkerRegistry",
	HandlerType: (*WorkerRegistryServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Register",
			Handler:    _WorkerRegistry_Register_Handler,
		},
		{
			MethodName: "Heartbeat",
			Handler:    _WorkerRegistry_Heartbeat_Handler,
		},
		{
			MethodName: "Deregister",
			Handler:    _WorkerRegistry_Deregister_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "agentos/v1/worker.proto",
}
