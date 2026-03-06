// Hand-written protobuf types for the WorkerRegistry service.
// These match the wire format from api/proto/agentos/v1/worker.proto
// and the Rust types in runtime/crates/worker/src/proto.rs.
//
// Regenerate with protoc when the .proto changes; this hand-written
// version avoids requiring the protoc toolchain during builds.

package v1

// RegisterRequest is sent by a worker to register itself with the control plane.
type RegisterRequest struct {
	WorkerId     string   `protobuf:"bytes,1,opt,name=worker_id,json=workerId,proto3" json:"worker_id,omitempty"`
	Addr         string   `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	Capabilities []string `protobuf:"bytes,3,rep,name=capabilities,proto3" json:"capabilities,omitempty"`
	MaxTasks     int32    `protobuf:"varint,4,opt,name=max_tasks,json=maxTasks,proto3" json:"max_tasks,omitempty"`
}

func (x *RegisterRequest) Reset()         { *x = RegisterRequest{} }
func (x *RegisterRequest) String() string { return x.WorkerId }
func (x *RegisterRequest) ProtoMessage()  {}

func (x *RegisterRequest) GetWorkerId() string {
	if x != nil {
		return x.WorkerId
	}
	return ""
}

func (x *RegisterRequest) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *RegisterRequest) GetCapabilities() []string {
	if x != nil {
		return x.Capabilities
	}
	return nil
}

func (x *RegisterRequest) GetMaxTasks() int32 {
	if x != nil {
		return x.MaxTasks
	}
	return 0
}

// RegisterResponse is returned after a worker registers.
type RegisterResponse struct {
	Accepted bool `protobuf:"varint,1,opt,name=accepted,proto3" json:"accepted,omitempty"`
}

func (x *RegisterResponse) Reset()         { *x = RegisterResponse{} }
func (x *RegisterResponse) String() string { return "" }
func (x *RegisterResponse) ProtoMessage()  {}

func (x *RegisterResponse) GetAccepted() bool {
	if x != nil {
		return x.Accepted
	}
	return false
}

// HeartbeatRequest is a periodic health ping from a worker.
type HeartbeatRequest struct {
	WorkerId    string `protobuf:"bytes,1,opt,name=worker_id,json=workerId,proto3" json:"worker_id,omitempty"`
	ActiveTasks int32  `protobuf:"varint,2,opt,name=active_tasks,json=activeTasks,proto3" json:"active_tasks,omitempty"`
}

func (x *HeartbeatRequest) Reset()         { *x = HeartbeatRequest{} }
func (x *HeartbeatRequest) String() string { return x.WorkerId }
func (x *HeartbeatRequest) ProtoMessage()  {}

func (x *HeartbeatRequest) GetWorkerId() string {
	if x != nil {
		return x.WorkerId
	}
	return ""
}

func (x *HeartbeatRequest) GetActiveTasks() int32 {
	if x != nil {
		return x.ActiveTasks
	}
	return 0
}

// HeartbeatResponse acknowledges a heartbeat.
type HeartbeatResponse struct {
	Ok bool `protobuf:"varint,1,opt,name=ok,proto3" json:"ok,omitempty"`
}

func (x *HeartbeatResponse) Reset()         { *x = HeartbeatResponse{} }
func (x *HeartbeatResponse) String() string { return "" }
func (x *HeartbeatResponse) ProtoMessage()  {}

func (x *HeartbeatResponse) GetOk() bool {
	if x != nil {
		return x.Ok
	}
	return false
}

// DeregisterRequest is sent by a worker to remove itself from the control plane.
type DeregisterRequest struct {
	WorkerId string `protobuf:"bytes,1,opt,name=worker_id,json=workerId,proto3" json:"worker_id,omitempty"`
}

func (x *DeregisterRequest) Reset()         { *x = DeregisterRequest{} }
func (x *DeregisterRequest) String() string { return x.WorkerId }
func (x *DeregisterRequest) ProtoMessage()  {}

func (x *DeregisterRequest) GetWorkerId() string {
	if x != nil {
		return x.WorkerId
	}
	return ""
}

// DeregisterResponse acknowledges a deregistration.
type DeregisterResponse struct {
	Ok bool `protobuf:"varint,1,opt,name=ok,proto3" json:"ok,omitempty"`
}

func (x *DeregisterResponse) Reset()         { *x = DeregisterResponse{} }
func (x *DeregisterResponse) String() string { return "" }
func (x *DeregisterResponse) ProtoMessage()  {}

func (x *DeregisterResponse) GetOk() bool {
	if x != nil {
		return x.Ok
	}
	return false
}
