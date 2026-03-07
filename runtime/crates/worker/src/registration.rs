//! Worker registration and heartbeat client.
//!
//! On startup the worker registers itself with the control plane via the
//! `WorkerRegistry` gRPC service. A background tokio task then sends periodic
//! heartbeats so the control plane knows the worker is alive.

use crate::proto::worker_registry_client::WorkerRegistryClient;
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

#[tonic::async_trait]
trait WorkerRegistryTransport: Send + Sync {
    async fn register(
        &self,
        control_plane_addr: &str,
        request: RegisterRequest,
    ) -> Result<RegisterResponse, RegistrationError>;

    async fn heartbeat(
        &self,
        control_plane_addr: &str,
        request: HeartbeatRequest,
    ) -> Result<HeartbeatResponse, RegistrationError>;

    async fn deregister(
        &self,
        control_plane_addr: &str,
        request: DeregisterRequest,
    ) -> Result<DeregisterResponse, RegistrationError>;
}

#[derive(Default)]
struct GrpcWorkerRegistryTransport;

#[tonic::async_trait]
impl WorkerRegistryTransport for GrpcWorkerRegistryTransport {
    async fn register(
        &self,
        control_plane_addr: &str,
        request: RegisterRequest,
    ) -> Result<RegisterResponse, RegistrationError> {
        let mut client =
            WorkerRegistryClient::connect(normalize_control_plane_addr(control_plane_addr)).await?;
        let resp = client.register(request).await?;
        Ok(resp.into_inner())
    }

    async fn heartbeat(
        &self,
        control_plane_addr: &str,
        request: HeartbeatRequest,
    ) -> Result<HeartbeatResponse, RegistrationError> {
        let mut client =
            WorkerRegistryClient::connect(normalize_control_plane_addr(control_plane_addr)).await?;
        let resp = client.heartbeat(request).await?;
        Ok(resp.into_inner())
    }

    async fn deregister(
        &self,
        control_plane_addr: &str,
        request: DeregisterRequest,
    ) -> Result<DeregisterResponse, RegistrationError> {
        let mut client =
            WorkerRegistryClient::connect(normalize_control_plane_addr(control_plane_addr)).await?;
        let resp = client.deregister(request).await?;
        Ok(resp.into_inner())
    }
}

/// A client that manages the worker's lifecycle with the control plane.
///
/// Holds the worker identity and provides `register`, `heartbeat`, and
/// `deregister` RPCs. Call [`start_heartbeat_loop`](Self::start_heartbeat_loop)
/// to spawn a background task that keeps the control plane updated.
pub struct RegistrationClient {
    worker_id: String,
    listen_addr: String,
    control_plane_addr: String,
    capabilities: Vec<String>,
    max_tasks: u32,
    transport: Arc<dyn WorkerRegistryTransport>,
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
        Self::with_transport(
            worker_id,
            listen_addr,
            control_plane_addr,
            capabilities,
            max_tasks,
            Arc::new(GrpcWorkerRegistryTransport),
        )
    }

    fn with_transport(
        worker_id: String,
        listen_addr: String,
        control_plane_addr: String,
        capabilities: Vec<String>,
        max_tasks: u32,
        transport: Arc<dyn WorkerRegistryTransport>,
    ) -> Self {
        Self {
            worker_id,
            listen_addr,
            control_plane_addr,
            capabilities,
            max_tasks,
            transport,
        }
    }

    pub fn worker_id(&self) -> &str {
        &self.worker_id
    }

    /// Register this worker with the control plane.
    pub async fn register(&self) -> Result<RegisterResponse, RegistrationError> {
        let inner = self
            .transport
            .register(
                &self.control_plane_addr,
                RegisterRequest {
                    worker_id: self.worker_id.clone(),
                    addr: self.listen_addr.clone(),
                    capabilities: self.capabilities.clone(),
                    max_tasks: self.max_tasks as i32,
                },
            )
            .await?;

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
        self.transport
            .heartbeat(
                &self.control_plane_addr,
                HeartbeatRequest {
                    worker_id: self.worker_id.clone(),
                    active_tasks: active_tasks as i32,
                },
            )
            .await
    }

    /// Deregister this worker from the control plane.
    pub async fn deregister(&self) -> Result<DeregisterResponse, RegistrationError> {
        self.transport
            .deregister(
                &self.control_plane_addr,
                DeregisterRequest {
                    worker_id: self.worker_id.clone(),
                },
            )
            .await
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
    use std::sync::Mutex;

    #[derive(Default)]
    struct MockTransportState {
        registers: Vec<(String, RegisterRequest)>,
        heartbeats: Vec<(String, HeartbeatRequest)>,
        deregisters: Vec<(String, DeregisterRequest)>,
        accept_registration: bool,
    }

    #[derive(Default)]
    struct MockTransport {
        state: Mutex<MockTransportState>,
    }

    impl MockTransport {
        fn accepting() -> Self {
            Self {
                state: Mutex::new(MockTransportState {
                    accept_registration: true,
                    ..MockTransportState::default()
                }),
            }
        }

        fn heartbeat_count(&self) -> usize {
            self.state.lock().unwrap().heartbeats.len()
        }
    }

    #[tonic::async_trait]
    impl WorkerRegistryTransport for MockTransport {
        async fn register(
            &self,
            control_plane_addr: &str,
            request: RegisterRequest,
        ) -> Result<RegisterResponse, RegistrationError> {
            let mut state = self.state.lock().unwrap();
            state
                .registers
                .push((control_plane_addr.to_string(), request));
            Ok(RegisterResponse {
                accepted: state.accept_registration,
            })
        }

        async fn heartbeat(
            &self,
            control_plane_addr: &str,
            request: HeartbeatRequest,
        ) -> Result<HeartbeatResponse, RegistrationError> {
            let mut state = self.state.lock().unwrap();
            state
                .heartbeats
                .push((control_plane_addr.to_string(), request));
            Ok(HeartbeatResponse { ok: true })
        }

        async fn deregister(
            &self,
            control_plane_addr: &str,
            request: DeregisterRequest,
        ) -> Result<DeregisterResponse, RegistrationError> {
            let mut state = self.state.lock().unwrap();
            state
                .deregisters
                .push((control_plane_addr.to_string(), request));
            Ok(DeregisterResponse { ok: true })
        }
    }

    #[tokio::test]
    async fn register_and_heartbeat_with_mock() {
        let transport = Arc::new(MockTransport::accepting());
        let client = RegistrationClient::with_transport(
            "test-worker-1".into(),
            "127.0.0.1:9999".into(),
            "127.0.0.1:50052".into(),
            vec!["shell".into()],
            4,
            transport.clone(),
        );

        let resp = client.register().await.expect("register should succeed");
        assert!(resp.accepted);

        let hb = client.heartbeat(2).await.expect("heartbeat should succeed");
        assert!(hb.ok);

        let dr = client
            .deregister()
            .await
            .expect("deregister should succeed");
        assert!(dr.ok);

        let state = transport.state.lock().unwrap();
        assert_eq!(state.registers.len(), 1);
        assert_eq!(state.heartbeats.len(), 1);
        assert_eq!(state.deregisters.len(), 1);
        assert_eq!(state.registers[0].0, "127.0.0.1:50052");
        assert_eq!(state.registers[0].1.worker_id, "test-worker-1");
        assert_eq!(state.registers[0].1.addr, "127.0.0.1:9999");
        assert_eq!(state.registers[0].1.capabilities, vec!["shell".to_string()]);
        assert_eq!(state.registers[0].1.max_tasks, 4);
        assert_eq!(state.heartbeats[0].1.active_tasks, 2);
        assert_eq!(state.deregisters[0].1.worker_id, "test-worker-1");
    }

    #[tokio::test]
    async fn register_returns_rejected_when_control_plane_denies() {
        let transport = Arc::new(MockTransport::default());
        let client = RegistrationClient::with_transport(
            "worker-rejected".into(),
            "127.0.0.1:9999".into(),
            "127.0.0.1:50052".into(),
            vec![],
            2,
            transport,
        );

        let err = client
            .register()
            .await
            .expect_err("register should be rejected");
        assert!(matches!(err, RegistrationError::Rejected));
    }

    #[tokio::test]
    async fn heartbeat_loop_sends_at_least_one() {
        let transport = Arc::new(MockTransport::accepting());
        let client = Arc::new(RegistrationClient::with_transport(
            "loop-worker".into(),
            "127.0.0.1:9999".into(),
            "127.0.0.1:50052".into(),
            vec![],
            2,
            transport.clone(),
        ));

        let active = Arc::new(AtomicU32::new(1));
        let handle = client.start_heartbeat_loop(Duration::from_millis(50), active);
        tokio::time::sleep(Duration::from_millis(200)).await;
        handle.abort();

        assert!(transport.heartbeat_count() >= 1);
    }

    #[test]
    fn registration_error_display() {
        let e = RegistrationError::Rejected;
        assert!(e.to_string().contains("rejected"));
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
