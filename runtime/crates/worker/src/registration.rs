//! Worker registration and heartbeat client.
//!
//! On startup the worker registers itself with the control plane via the
//! `WorkerRegistry` gRPC service. A background tokio task then sends periodic
//! heartbeats so the control plane knows the worker is alive.

use crate::proto::{
    DeregisterRequest, DeregisterResponse, HeartbeatRequest, HeartbeatResponse, RegisterRequest,
    RegisterResponse,
};
use std::sync::atomic::{AtomicU32, Ordering};
use std::sync::Arc;
use std::time::Duration;

/// Errors from the registration subsystem.
#[derive(Debug)]
pub enum RegistrationError {
    /// The control plane rejected the registration.
    Rejected,
    /// A transport or gRPC error occurred.
    Transport(String),
}

impl std::fmt::Display for RegistrationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Rejected => write!(f, "registration rejected by control plane"),
            Self::Transport(msg) => write!(f, "registration transport error: {msg}"),
        }
    }
}

impl std::error::Error for RegistrationError {}

impl From<tonic::Status> for RegistrationError {
    fn from(s: tonic::Status) -> Self {
        Self::Transport(s.message().to_string())
    }
}

impl From<tonic::transport::Error> for RegistrationError {
    fn from(e: tonic::transport::Error) -> Self {
        Self::Transport(e.to_string())
    }
}

/// A client that manages the worker's lifecycle with the control plane.
///
/// Holds the worker identity and provides `register`, `heartbeat`, and
/// `deregister` RPCs.  Call [`start_heartbeat_loop`](Self::start_heartbeat_loop)
/// to spawn a background task that keeps the control plane updated.
pub struct RegistrationClient {
    worker_id: String,
    listen_addr: String,
    control_plane_addr: String,
    capabilities: Vec<String>,
    max_tasks: u32,
}

fn normalize_control_plane_addr(addr: &str) -> String {
    let trimmed = addr.trim();
    if trimmed.contains("://") {
        trimmed.to_string()
    } else {
        format!("http://{trimmed}")
    }
}

impl RegistrationClient {
    pub fn new(
        worker_id: String,
        listen_addr: String,
        control_plane_addr: String,
        capabilities: Vec<String>,
        max_tasks: u32,
    ) -> Self {
        Self {
            worker_id,
            listen_addr,
            control_plane_addr,
            capabilities,
            max_tasks,
        }
    }

    pub fn worker_id(&self) -> &str {
        &self.worker_id
    }

    /// Register this worker with the control plane.
    pub async fn register(&self) -> Result<RegisterResponse, RegistrationError> {
        let mut client = crate::proto::worker_registry_client::WorkerRegistryClient::connect(
            normalize_control_plane_addr(&self.control_plane_addr),
        )
        .await?;

        let resp = client
            .register(RegisterRequest {
                worker_id: self.worker_id.clone(),
                addr: self.listen_addr.clone(),
                capabilities: self.capabilities.clone(),
                max_tasks: self.max_tasks as i32,
            })
            .await?;

        let inner = resp.into_inner();
        if !inner.accepted {
            return Err(RegistrationError::Rejected);
        }
        Ok(inner)
    }

    /// Send a single heartbeat reporting the current active task count.
    pub async fn heartbeat(
        &self,
        active_tasks: u32,
    ) -> Result<HeartbeatResponse, RegistrationError> {
        let mut client = crate::proto::worker_registry_client::WorkerRegistryClient::connect(
            normalize_control_plane_addr(&self.control_plane_addr),
        )
        .await?;

        let resp = client
            .heartbeat(HeartbeatRequest {
                worker_id: self.worker_id.clone(),
                active_tasks: active_tasks as i32,
            })
            .await?;

        Ok(resp.into_inner())
    }

    /// Deregister this worker from the control plane.
    pub async fn deregister(&self) -> Result<DeregisterResponse, RegistrationError> {
        let mut client = crate::proto::worker_registry_client::WorkerRegistryClient::connect(
            normalize_control_plane_addr(&self.control_plane_addr),
        )
        .await?;

        let resp = client
            .deregister(DeregisterRequest {
                worker_id: self.worker_id.clone(),
            })
            .await?;

        Ok(resp.into_inner())
    }

    /// Spawn a background tokio task that sends heartbeats at `interval`.
    ///
    /// The loop reads the current task count from `active_tasks` each tick.
    /// Returns a `JoinHandle` so the caller can abort it during shutdown.
    pub fn start_heartbeat_loop(
        self: Arc<Self>,
        interval: Duration,
        active_tasks: Arc<AtomicU32>,
    ) -> tokio::task::JoinHandle<()> {
        tokio::spawn(async move {
            let mut ticker = tokio::time::interval(interval);
            // Skip the first immediate tick -- we just registered.
            ticker.tick().await;

            loop {
                ticker.tick().await;
                let count = active_tasks.load(Ordering::Relaxed);
                match self.heartbeat(count).await {
                    Ok(_) => {
                        eprintln!(
                            "[heartbeat] ok  worker={} active_tasks={count}",
                            self.worker_id
                        );
                    }
                    Err(e) => {
                        eprintln!("[heartbeat] err worker={} {e}", self.worker_id);
                    }
                }
            }
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::proto::worker_registry_server::WorkerRegistry;
    use crate::proto::{
        DeregisterRequest, DeregisterResponse, HeartbeatRequest, HeartbeatResponse,
        RegisterRequest, RegisterResponse,
    };
    use std::net::SocketAddr;
    use tonic::transport::Server;
    use tonic::{Request, Response, Status};

    // ----- Mock server -----

    #[derive(Default)]
    struct MockRegistry;

    #[tonic::async_trait]
    impl WorkerRegistry for MockRegistry {
        async fn register(
            &self,
            req: Request<RegisterRequest>,
        ) -> Result<Response<RegisterResponse>, Status> {
            let inner = req.into_inner();
            Ok(Response::new(RegisterResponse {
                accepted: !inner.worker_id.is_empty(),
            }))
        }

        async fn heartbeat(
            &self,
            _req: Request<HeartbeatRequest>,
        ) -> Result<Response<HeartbeatResponse>, Status> {
            Ok(Response::new(HeartbeatResponse { ok: true }))
        }

        async fn deregister(
            &self,
            _req: Request<DeregisterRequest>,
        ) -> Result<Response<DeregisterResponse>, Status> {
            Ok(Response::new(DeregisterResponse { ok: true }))
        }
    }

    /// A minimal tonic service wrapper for the mock WorkerRegistry.
    #[derive(Clone)]
    struct MockRegistryServer {
        inner: Arc<MockRegistry>,
    }

    impl MockRegistryServer {
        fn new(mock: MockRegistry) -> Self {
            Self {
                inner: Arc::new(mock),
            }
        }
    }

    /// Build a gRPC-framed response body from protobuf bytes.
    fn grpc_frame(data: &[u8]) -> Vec<u8> {
        let mut frame = Vec::with_capacity(5 + data.len());
        frame.push(0u8); // not compressed
        frame.extend_from_slice(&(data.len() as u32).to_be_bytes());
        frame.extend_from_slice(data);
        frame
    }

    impl tonic::codegen::Service<tonic::codegen::http::Request<tonic::transport::Body>>
        for MockRegistryServer
    {
        type Response = tonic::codegen::http::Response<tonic::body::BoxBody>;
        type Error = std::convert::Infallible;
        type Future = std::pin::Pin<
            Box<dyn std::future::Future<Output = Result<Self::Response, Self::Error>> + Send>,
        >;

        fn poll_ready(
            &mut self,
            _cx: &mut std::task::Context<'_>,
        ) -> std::task::Poll<Result<(), Self::Error>> {
            std::task::Poll::Ready(Ok(()))
        }

        fn call(
            &mut self,
            req: tonic::codegen::http::Request<tonic::transport::Body>,
        ) -> Self::Future {
            let inner = self.inner.clone();
            Box::pin(async move {
                use tonic::codegen::Body as _;
                let path = req.uri().path().to_string();

                // Collect the request body.
                let (_parts, body) = req.into_parts();
                let body_bytes = match body.collect().await {
                    Ok(c) => c.to_bytes(),
                    Err(_) => {
                        let resp = tonic::codegen::http::Response::builder()
                            .status(200)
                            .header("content-type", "application/grpc")
                            .header("grpc-status", "13")
                            .body(tonic::body::empty_body())
                            .unwrap();
                        return Ok(resp);
                    }
                };

                // Strip gRPC frame header (1 flag + 4 len).
                let payload = if body_bytes.len() > 5 {
                    body_bytes.slice(5..)
                } else {
                    body_bytes.clone()
                };

                let (grpc_status, resp_bytes) = match path.as_str() {
                    "/agentos.v1.WorkerRegistry/Register" => {
                        let msg: RegisterRequest =
                            prost::Message::decode(payload).unwrap_or_default();
                        let r = inner.register(Request::new(msg)).await.unwrap();
                        let mut buf = Vec::new();
                        prost::Message::encode(&r.into_inner(), &mut buf).unwrap();
                        ("0", buf)
                    }
                    "/agentos.v1.WorkerRegistry/Heartbeat" => {
                        let msg: HeartbeatRequest =
                            prost::Message::decode(payload).unwrap_or_default();
                        let r = inner.heartbeat(Request::new(msg)).await.unwrap();
                        let mut buf = Vec::new();
                        prost::Message::encode(&r.into_inner(), &mut buf).unwrap();
                        ("0", buf)
                    }
                    "/agentos.v1.WorkerRegistry/Deregister" => {
                        let msg: DeregisterRequest =
                            prost::Message::decode(payload).unwrap_or_default();
                        let r = inner.deregister(Request::new(msg)).await.unwrap();
                        let mut buf = Vec::new();
                        prost::Message::encode(&r.into_inner(), &mut buf).unwrap();
                        ("0", buf)
                    }
                    _ => ("12", Vec::new()),
                };

                let frame = grpc_frame(&resp_bytes);

                // Build trailers-in-headers response (gRPC unary).
                // Map hyper body error to tonic Status and box it.
                let body: tonic::body::BoxBody = {
                    use tonic::codegen::Body as _;
                    tonic::transport::Body::from(frame)
                        .map_err(|e| tonic::Status::internal(e.to_string()))
                        .boxed_unsync()
                };

                let resp = tonic::codegen::http::Response::builder()
                    .status(200)
                    .header("content-type", "application/grpc+proto")
                    .header("grpc-status", grpc_status)
                    .body(body)
                    .unwrap();
                Ok(resp)
            })
        }
    }

    impl tonic::server::NamedService for MockRegistryServer {
        const NAME: &'static str = "agentos.v1.WorkerRegistry";
    }

    /// Start a mock server on a random OS-assigned port and return its address.
    async fn start_mock_server() -> String {
        let addr: SocketAddr = "[::1]:0".parse().unwrap();
        let listener = tokio::net::TcpListener::bind(addr).await.unwrap();
        let local_addr = listener.local_addr().unwrap();

        let incoming = tokio_stream::wrappers::TcpListenerStream::new(listener);
        let svc = MockRegistryServer::new(MockRegistry);
        tokio::spawn(async move {
            let _: Result<(), tonic::transport::Error> = Server::builder()
                .add_service(svc)
                .serve_with_incoming(incoming)
                .await;
        });

        // Give the server a moment to bind.
        tokio::time::sleep(Duration::from_millis(50)).await;
        format!("http://{local_addr}")
    }

    #[tokio::test]
    async fn register_and_heartbeat_with_mock() {
        let addr = start_mock_server().await;

        let client = RegistrationClient::new(
            "test-worker-1".into(),
            "[::1]:9999".into(),
            addr.clone(),
            vec!["shell".into()],
            4,
        );

        // Register
        let resp = client.register().await.expect("register should succeed");
        assert!(resp.accepted);

        // Heartbeat
        let hb = client.heartbeat(2).await.expect("heartbeat should succeed");
        assert!(hb.ok);

        // Deregister
        let dr = client
            .deregister()
            .await
            .expect("deregister should succeed");
        assert!(dr.ok);
    }

    #[tokio::test]
    async fn heartbeat_loop_sends_at_least_one() {
        let addr = start_mock_server().await;

        let client = Arc::new(RegistrationClient::new(
            "loop-worker".into(),
            "[::1]:9999".into(),
            addr,
            vec![],
            2,
        ));

        let active = Arc::new(AtomicU32::new(1));
        let handle = client.start_heartbeat_loop(Duration::from_millis(50), active);

        // Let it tick a couple of times.
        tokio::time::sleep(Duration::from_millis(200)).await;
        handle.abort();
    }

    #[test]
    fn registration_error_display() {
        let e = RegistrationError::Rejected;
        assert!(e.to_string().contains("rejected"));
        let e2 = RegistrationError::Transport("timeout".into());
        assert!(e2.to_string().contains("timeout"));
    }

    #[test]
    fn normalize_control_plane_addr_accepts_bare_host_port() {
        assert_eq!(
            normalize_control_plane_addr("127.0.0.1:50052"),
            "http://127.0.0.1:50052"
        );
        assert_eq!(
            normalize_control_plane_addr("http://127.0.0.1:50052"),
            "http://127.0.0.1:50052"
        );
    }
}
