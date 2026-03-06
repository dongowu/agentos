use agentos_sandbox::{IsolationProvider, SandboxHandle, SandboxSpec};

/// WorkerService owns the execution-facing worker lifecycle.
///
/// In later phases this will be wrapped by a gRPC service implementation.
pub struct WorkerService<P> {
    provider: P,
}

impl<P> WorkerService<P>
where
    P: IsolationProvider,
{
    pub fn new(provider: P) -> Self {
        Self { provider }
    }

    pub async fn acquire_runtime(
        &self,
        spec: SandboxSpec,
    ) -> Result<SandboxHandle, agentos_sandbox::SandboxError> {
        self.provider.start(spec).await
    }

    pub async fn release_runtime(
        &self,
        sandbox_id: &str,
    ) -> Result<(), agentos_sandbox::SandboxError> {
        self.provider.stop(sandbox_id).await
    }
}
