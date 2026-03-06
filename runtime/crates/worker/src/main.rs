use agentos_sandbox::factory::create_runtime;
use agentos_sandbox::security::SecurityPolicy;
use agentos_worker::config::WorkerConfig;
use agentos_worker::executor::ActionExecutor;
use agentos_worker::grpc;
use agentos_worker::registration::RegistrationClient;
use std::net::SocketAddr;
use std::sync::atomic::AtomicU32;
use std::sync::Arc;
use tokio_stream::wrappers::TcpListenerStream;
use tonic::transport::Server;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let config = WorkerConfig::from_env();

    let runtime =
        create_runtime(&config.runtime).map_err(|e| format!("failed to create runtime: {e}"))?;

    let security = Arc::new(config.security);
    let executor = Arc::new(ActionExecutor::new(Arc::from(runtime), security));

    let addr: SocketAddr = config.listen_addr.parse()?;
    let svc = grpc::RuntimeServiceImpl::new(executor).into_server();
    let listener = tokio::net::TcpListener::bind(addr).await?;
    let local_addr = listener.local_addr()?;

    eprintln!(
        "agentos-worker listening on {} (runtime={}, autonomy={:?}, worker_id={})",
        local_addr,
        config.runtime.kind,
        SecurityPolicy::default().autonomy,
        config.worker_id,
    );

    let _heartbeat_handle = if let Some(ref cp_addr) = config.control_plane_addr {
        let reg_client = Arc::new(RegistrationClient::new(
            config.worker_id.clone(),
            local_addr.to_string(),
            cp_addr.clone(),
            vec!["shell".into()],
            config.max_concurrent_tasks,
        ));

        match reg_client.register().await {
            Ok(resp) => {
                eprintln!(
                    "registered with control plane at {cp_addr} (accepted={})",
                    resp.accepted
                );
            }
            Err(e) => {
                eprintln!("warning: failed to register with control plane: {e}");
            }
        }

        let active_tasks = Arc::new(AtomicU32::new(0));
        let interval = std::time::Duration::from_secs(config.heartbeat_interval_secs);
        let handle = reg_client.start_heartbeat_loop(interval, active_tasks);
        Some(handle)
    } else {
        eprintln!("no AGENTOS_CONTROL_PLANE_ADDR set; skipping registration");
        None
    };

    Server::builder()
        .add_service(svc)
        .serve_with_incoming(TcpListenerStream::new(listener))
        .await?;

    Ok(())
}
