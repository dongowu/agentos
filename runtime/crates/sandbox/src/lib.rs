//! Isolation abstractions for AgentOS runtime.
//!
//! Implementations: DockerProvider, GVisorProvider, FirecrackerProvider (future).

use async_trait::async_trait;
use std::error::Error;

/// Specification for a sandbox environment.
#[derive(Debug, Clone, Default)]
pub struct SandboxSpec {
    pub profile: String,
}

/// Handle to a running sandbox.
#[derive(Debug, Clone)]
pub struct SandboxHandle {
    pub id: String,
    pub status: String,
}

/// Isolation backend.
#[async_trait]
pub trait IsolationProvider: Send + Sync {
    async fn start(&self, spec: SandboxSpec) -> Result<SandboxHandle, SandboxError>;
    async fn stop(&self, sandbox_id: &str) -> Result<(), SandboxError>;
}

/// Docker-based isolation (MVP).
pub struct DockerProvider;

impl DockerProvider {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait]
impl IsolationProvider for DockerProvider {
    async fn start(&self, spec: SandboxSpec) -> Result<SandboxHandle, SandboxError> {
        let _ = spec;
        Ok(SandboxHandle {
            id: "stub".to_string(),
            status: "started".to_string(),
        })
    }

    async fn stop(&self, _sandbox_id: &str) -> Result<(), SandboxError> {
        Ok(())
    }
}

#[derive(Debug)]
pub struct SandboxError;

impl std::fmt::Display for SandboxError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "SandboxError")
    }
}

impl Error for SandboxError {}
