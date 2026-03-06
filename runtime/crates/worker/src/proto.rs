// Generated from api/proto/agentos/v1/runtime.proto
// This file is checked in so that `protoc` is not required for building.
// Regenerate with: protoc + tonic-build when the .proto changes.

/// Request to execute a single action in a sandbox.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ExecuteActionRequest {
    #[prost(string, tag = "1")]
    pub task_id: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub action_id: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub runtime_profile: ::prost::alloc::string::String,
    #[prost(bytes = "vec", tag = "4")]
    pub payload: ::prost::alloc::vec::Vec<u8>,
}

/// Response from action execution.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ExecuteActionResponse {
    #[prost(int32, tag = "1")]
    pub exit_code: i32,
    #[prost(bytes = "vec", tag = "2")]
    pub stdout: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes = "vec", tag = "3")]
    pub stderr: ::prost::alloc::vec::Vec<u8>,
}

/// Request to stream output from a running action.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct StreamOutputRequest {
    #[prost(string, tag = "1")]
    pub task_id: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub action_id: ::prost::alloc::string::String,
}

/// A chunk of streaming output data.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct StreamChunk {
    #[prost(string, tag = "1")]
    pub task_id: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub action_id: ::prost::alloc::string::String,
    #[prost(bytes = "vec", tag = "3")]
    pub data: ::prost::alloc::vec::Vec<u8>,
    /// stdout, stderr, resource
    #[prost(string, tag = "4")]
    pub kind: ::prost::alloc::string::String,
}

// ---------------------------------------------------------------------------
// Worker registration & heartbeat messages
// ---------------------------------------------------------------------------

/// Request to register a worker with the control plane.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct RegisterRequest {
    #[prost(string, tag = "1")]
    pub worker_id: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub addr: ::prost::alloc::string::String,
    #[prost(string, repeated, tag = "3")]
    pub capabilities: ::prost::alloc::vec::Vec<::prost::alloc::string::String>,
    #[prost(int32, tag = "4")]
    pub max_tasks: i32,
}

/// Response from worker registration.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct RegisterResponse {
    #[prost(bool, tag = "1")]
    pub accepted: bool,
}

/// Periodic heartbeat from a worker.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct HeartbeatRequest {
    #[prost(string, tag = "1")]
    pub worker_id: ::prost::alloc::string::String,
    #[prost(int32, tag = "2")]
    pub active_tasks: i32,
}

/// Response to a heartbeat.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct HeartbeatResponse {
    #[prost(bool, tag = "1")]
    pub ok: bool,
}

/// Request to deregister a worker from the control plane.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct DeregisterRequest {
    #[prost(string, tag = "1")]
    pub worker_id: ::prost::alloc::string::String,
}

/// Response from deregistration.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct DeregisterResponse {
    #[prost(bool, tag = "1")]
    pub ok: bool,
}

/// Generated client for the WorkerRegistry service on the control plane.
pub mod worker_registry_client {
    use super::*;

    #[derive(Debug, Clone)]
    pub struct WorkerRegistryClient<T> {
        inner: T,
    }

    impl WorkerRegistryClient<tonic::transport::Channel> {
        /// Connect to a WorkerRegistry gRPC server at `dst`.
        pub async fn connect(
            dst: impl TryInto<tonic::transport::Endpoint, Error: Into<Box<dyn std::error::Error + Send + Sync>>>,
        ) -> Result<Self, tonic::transport::Error> {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self { inner: conn })
        }
    }

    impl<T> WorkerRegistryClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::BoxBody> + Clone,
        T::Error: Into<Box<dyn std::error::Error + Send + Sync>>,
        T::ResponseBody: tonic::codegen::Body<Data = tonic::codegen::Bytes> + Send + 'static,
        <T::ResponseBody as tonic::codegen::Body>::Error: Into<Box<dyn std::error::Error + Send + Sync>> + Send,
    {
        pub fn new(inner: T) -> Self {
            Self { inner }
        }

        pub async fn register(
            &mut self,
            request: impl tonic::IntoRequest<RegisterRequest>,
        ) -> std::result::Result<tonic::Response<RegisterResponse>, tonic::Status> {
            let codec = tonic::codec::ProstCodec::default();
            let path = tonic::codegen::http::uri::PathAndQuery::from_static(
                "/agentos.v1.WorkerRegistry/Register",
            );
            let mut grpc = tonic::client::Grpc::new(self.inner.clone());
            grpc.ready().await.map_err(|e| {
                tonic::Status::new(tonic::Code::Unknown, format!("Service not ready: {}", e.into()))
            })?;
            grpc.unary(request.into_request(), path, codec).await
        }

        pub async fn heartbeat(
            &mut self,
            request: impl tonic::IntoRequest<HeartbeatRequest>,
        ) -> std::result::Result<tonic::Response<HeartbeatResponse>, tonic::Status> {
            let codec = tonic::codec::ProstCodec::default();
            let path = tonic::codegen::http::uri::PathAndQuery::from_static(
                "/agentos.v1.WorkerRegistry/Heartbeat",
            );
            let mut grpc = tonic::client::Grpc::new(self.inner.clone());
            grpc.ready().await.map_err(|e| {
                tonic::Status::new(tonic::Code::Unknown, format!("Service not ready: {}", e.into()))
            })?;
            grpc.unary(request.into_request(), path, codec).await
        }

        pub async fn deregister(
            &mut self,
            request: impl tonic::IntoRequest<DeregisterRequest>,
        ) -> std::result::Result<tonic::Response<DeregisterResponse>, tonic::Status> {
            let codec = tonic::codec::ProstCodec::default();
            let path = tonic::codegen::http::uri::PathAndQuery::from_static(
                "/agentos.v1.WorkerRegistry/Deregister",
            );
            let mut grpc = tonic::client::Grpc::new(self.inner.clone());
            grpc.ready().await.map_err(|e| {
                tonic::Status::new(tonic::Code::Unknown, format!("Service not ready: {}", e.into()))
            })?;
            grpc.unary(request.into_request(), path, codec).await
        }
    }
}

/// Generated server trait for WorkerRegistry (implemented by the control plane).
pub mod worker_registry_server {
    use super::*;

    #[tonic::async_trait]
    pub trait WorkerRegistry: Send + Sync + 'static {
        async fn register(
            &self,
            request: tonic::Request<RegisterRequest>,
        ) -> std::result::Result<tonic::Response<RegisterResponse>, tonic::Status>;

        async fn heartbeat(
            &self,
            request: tonic::Request<HeartbeatRequest>,
        ) -> std::result::Result<tonic::Response<HeartbeatResponse>, tonic::Status>;

        async fn deregister(
            &self,
            request: tonic::Request<DeregisterRequest>,
        ) -> std::result::Result<tonic::Response<DeregisterResponse>, tonic::Status>;
    }
}

/// Generated server implementations.
pub mod runtime_service_server {
    use super::*;

    #[tonic::async_trait]
    pub trait RuntimeService: Send + Sync + 'static {
        async fn execute_action(
            &self,
            request: tonic::Request<ExecuteActionRequest>,
        ) -> std::result::Result<tonic::Response<ExecuteActionResponse>, tonic::Status>;

        type StreamOutputStream: futures::Stream<Item = std::result::Result<StreamChunk, tonic::Status>>
            + Send
            + 'static;

        async fn stream_output(
            &self,
            request: tonic::Request<StreamOutputRequest>,
        ) -> std::result::Result<tonic::Response<Self::StreamOutputStream>, tonic::Status>;
    }

    #[derive(Debug)]
    pub struct RuntimeServiceServer<T: RuntimeService> {
        inner: std::sync::Arc<T>,
    }

    impl<T: RuntimeService> RuntimeServiceServer<T> {
        pub fn new(inner: T) -> Self {
            Self {
                inner: std::sync::Arc::new(inner),
            }
        }
    }

    impl<T: RuntimeService> Clone for RuntimeServiceServer<T> {
        fn clone(&self) -> Self {
            Self {
                inner: self.inner.clone(),
            }
        }
    }

    // Service impl matches tonic 0.11's add_service signature:
    // Service<Request<Body>, Response = Response<BoxBody>, Error = Infallible>
    impl<T: RuntimeService> tonic::codegen::Service<tonic::codegen::http::Request<tonic::transport::Body>>
        for RuntimeServiceServer<T>
    {
        type Response = tonic::codegen::http::Response<tonic::body::BoxBody>;
        type Error = std::convert::Infallible;
        type Future = std::pin::Pin<
            Box<dyn std::future::Future<Output = std::result::Result<Self::Response, Self::Error>> + Send>,
        >;

        fn poll_ready(
            &mut self,
            _cx: &mut std::task::Context<'_>,
        ) -> std::task::Poll<std::result::Result<(), Self::Error>> {
            std::task::Poll::Ready(Ok(()))
        }

        fn call(
            &mut self,
            _req: tonic::codegen::http::Request<tonic::transport::Body>,
        ) -> Self::Future {
            // Wire-level gRPC frame decoding is handled by tonic in production
            // via protoc-generated code. This hand-written server returns
            // UNIMPLEMENTED for wire-level calls. The RuntimeService trait
            // methods are tested directly in grpc.rs tests.
            Box::pin(async {
                let resp = tonic::codegen::http::Response::builder()
                    .status(200)
                    .header("content-type", "application/grpc")
                    .header("grpc-status", "12") // UNIMPLEMENTED
                    .header("grpc-message", "use protoc-generated server for wire gRPC")
                    .body(tonic::body::empty_body())
                    .unwrap();
                Ok(resp)
            })
        }
    }

    impl<T: RuntimeService> tonic::server::NamedService for RuntimeServiceServer<T> {
        const NAME: &'static str = "agentos.v1.RuntimeService";
    }
}
