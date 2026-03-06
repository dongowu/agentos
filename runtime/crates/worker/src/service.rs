//! Legacy WorkerService -- delegates to the old IsolationProvider interface.
//!
//! Kept for backward compatibility. New code should use `executor::ActionExecutor`.

#[allow(deprecated)]
use agentos_sandbox::{IsolationProvider, SandboxHandle, SandboxSpec};

/// WorkerService owns the execution-facing worker lifecycle.
///
/// **Deprecated:** Use [`crate::executor::ActionExecutor`] instead.
#[deprecated(note = "Use executor::ActionExecutor instead")]
pub struct WorkerService<P> {
    provider: P,
}

#[allow(deprecated)]
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
