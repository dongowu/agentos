use agentos_sandbox::{DockerProvider, IsolationProvider, SandboxSpec};

#[tokio::test]
async fn docker_provider_returns_started_handle() {
    let provider = DockerProvider::new();
    let handle = provider.start(SandboxSpec::default()).await.expect("start");
    assert_eq!(handle.status, "started");
}
