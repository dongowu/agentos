//! Telemetry models and stream helpers for AgentOS runtime.

/// Stream chunk kinds.
pub const KIND_STDOUT: &str = "stdout";
pub const KIND_STDERR: &str = "stderr";
pub const KIND_RESOURCE: &str = "resource";

/// ResourceUsage is the normalized runtime usage snapshot.
#[derive(Debug, Clone, Default)]
pub struct ResourceUsage {
    pub cpu_millis: u64,
    pub memory_bytes: u64,
}

/// StreamChunk is the cross-runtime streaming payload.
#[derive(Debug, Clone, Default)]
pub struct StreamChunk {
    pub kind: String,
    pub data: Vec<u8>,
}
