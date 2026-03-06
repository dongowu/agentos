package v1

// ListWorkersRequest requests the current worker snapshots from the control plane.
type ListWorkersRequest struct {
	AvailableOnly bool `protobuf:"varint,1,opt,name=available_only,json=availableOnly,proto3" json:"available_only,omitempty"`
}

func (x *ListWorkersRequest) Reset()         { *x = ListWorkersRequest{} }
func (x *ListWorkersRequest) String() string { return "" }
func (x *ListWorkersRequest) ProtoMessage()  {}

func (x *ListWorkersRequest) GetAvailableOnly() bool {
	if x != nil {
		return x.AvailableOnly
	}
	return false
}

// WorkerSnapshot is a serialized view of WorkerInfo for remote scheduling.
type WorkerSnapshot struct {
	WorkerId          string   `protobuf:"bytes,1,opt,name=worker_id,json=workerId,proto3" json:"worker_id,omitempty"`
	Addr              string   `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	Capabilities      []string `protobuf:"bytes,3,rep,name=capabilities,proto3" json:"capabilities,omitempty"`
	Status            string   `protobuf:"bytes,4,opt,name=status,proto3" json:"status,omitempty"`
	ActiveTasks       int32    `protobuf:"varint,5,opt,name=active_tasks,json=activeTasks,proto3" json:"active_tasks,omitempty"`
	MaxTasks          int32    `protobuf:"varint,6,opt,name=max_tasks,json=maxTasks,proto3" json:"max_tasks,omitempty"`
	LastHeartbeatUnix int64    `protobuf:"varint,7,opt,name=last_heartbeat_unix,json=lastHeartbeatUnix,proto3" json:"last_heartbeat_unix,omitempty"`
}

func (x *WorkerSnapshot) Reset()         { *x = WorkerSnapshot{} }
func (x *WorkerSnapshot) String() string { return x.WorkerId }
func (x *WorkerSnapshot) ProtoMessage()  {}

func (x *WorkerSnapshot) GetWorkerId() string {
	if x != nil {
		return x.WorkerId
	}
	return ""
}

func (x *WorkerSnapshot) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *WorkerSnapshot) GetCapabilities() []string {
	if x != nil {
		return x.Capabilities
	}
	return nil
}

func (x *WorkerSnapshot) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *WorkerSnapshot) GetActiveTasks() int32 {
	if x != nil {
		return x.ActiveTasks
	}
	return 0
}

func (x *WorkerSnapshot) GetMaxTasks() int32 {
	if x != nil {
		return x.MaxTasks
	}
	return 0
}

func (x *WorkerSnapshot) GetLastHeartbeatUnix() int64 {
	if x != nil {
		return x.LastHeartbeatUnix
	}
	return 0
}

// ListWorkersResponse returns the current set of worker snapshots.
type ListWorkersResponse struct {
	Workers []*WorkerSnapshot `protobuf:"bytes,1,rep,name=workers,proto3" json:"workers,omitempty"`
}

func (x *ListWorkersResponse) Reset()         { *x = ListWorkersResponse{} }
func (x *ListWorkersResponse) String() string { return "" }
func (x *ListWorkersResponse) ProtoMessage()  {}

func (x *ListWorkersResponse) GetWorkers() []*WorkerSnapshot {
	if x != nil {
		return x.Workers
	}
	return nil
}
