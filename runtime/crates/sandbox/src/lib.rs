//! Sandbox crate for AgentOS runtime.
//!
//! Provides the `RuntimeAdapter` trait for abstracting shell execution across
//! native, Docker, and future runtimes. Includes security policy enforcement,
//! secret redaction, and output truncation.

pub mod docker;
pub mod factory;
pub mod native;
pub mod security;

use async_trait::async_trait;
use std::collections::HashMap;
use std::path::PathBuf;
use std::time::Duration;

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

/// Specification for a single command execution.
#[derive(Debug, Clone)]
pub struct ExecutionSpec {
    /// Shell command string to execute.
    pub command: String,
    /// Working directory. If `None`, uses the runtime default.
    pub working_dir: Option<PathBuf>,
    /// Extra environment variables to inject (merged after isolation).
    pub env: HashMap<String, String>,
    /// Maximum wall-clock time before the command is killed.
    pub timeout: Duration,
    /// Maximum combined stdout+stderr bytes before truncation.
    pub max_output_bytes: usize,
}

impl Default for ExecutionSpec {
    fn default() -> Self {
        Self {
            command: String::new(),
            working_dir: None,
            env: HashMap::new(),
            timeout: Duration::from_secs(60),
            max_output_bytes: 1_048_576, // 1 MB
        }
    }
}

/// Result of a command execution.
#[derive(Debug, Clone)]
pub struct ExecutionResult {
    /// Process exit code (non-zero indicates failure).
    pub exit_code: i32,
    /// Captured stdout bytes (may be truncated).
    pub stdout: Vec<u8>,
    /// Captured stderr bytes (may be truncated).
    pub stderr: Vec<u8>,
    /// Actual wall-clock duration of execution.
    pub duration: Duration,
    /// Whether the output was truncated due to `max_output_bytes`.
    pub truncated: bool,
}

/// Errors that can occur during runtime operations.
#[derive(Debug)]
pub enum RuntimeError {
    /// The command exceeded its timeout.
    Timeout { elapsed: Duration },
    /// The shell binary was not found on this platform.
    ShellNotFound(String),
    /// An I/O error during command execution.
    IoError(std::io::Error),
    /// A security policy violation.
    SecurityViolation(String),
    /// Generic runtime error.
    Other(String),
}

impl std::fmt::Display for RuntimeError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            RuntimeError::Timeout { elapsed } => {
                write!(f, "command timed out after {:.1}s", elapsed.as_secs_f64())
            }
            RuntimeError::ShellNotFound(msg) => write!(f, "shell not found: {msg}"),
            RuntimeError::IoError(e) => write!(f, "I/O error: {e}"),
            RuntimeError::SecurityViolation(msg) => write!(f, "security violation: {msg}"),
            RuntimeError::Other(msg) => write!(f, "{msg}"),
        }
    }
}

impl std::error::Error for RuntimeError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            RuntimeError::IoError(e) => Some(e),
            _ => None,
        }
    }
}

impl From<std::io::Error> for RuntimeError {
    fn from(e: std::io::Error) -> Self {
        RuntimeError::IoError(e)
    }
}

// ---------------------------------------------------------------------------
// RuntimeAdapter trait
// ---------------------------------------------------------------------------

/// Abstracts platform differences for command execution.
///
/// Implementations handle shell detection, environment isolation, timeout
/// enforcement, output truncation, and secret redaction.
#[async_trait]
pub trait RuntimeAdapter: Send + Sync {
    /// Human-readable name for this runtime (e.g. "native", "docker").
    fn name(&self) -> &str;

    /// Whether this runtime can execute shell commands.
    fn has_shell_access(&self) -> bool;

    /// Whether this runtime has filesystem read/write access.
    fn has_filesystem_access(&self) -> bool;

    /// Memory budget in bytes. 0 means unlimited.
    fn memory_budget(&self) -> u64 {
        0
    }

    /// Execute a command according to the given spec.
    async fn execute(&self, spec: ExecutionSpec) -> Result<ExecutionResult, RuntimeError>;
}

// ---------------------------------------------------------------------------
// Backward-compatible deprecated types
// ---------------------------------------------------------------------------

/// Specification for a sandbox environment.
#[deprecated(note = "Use ExecutionSpec + RuntimeAdapter instead")]
#[derive(Debug, Clone, Default)]
pub struct SandboxSpec {
    pub profile: String,
}

/// Handle to a running sandbox.
#[deprecated(note = "Use ExecutionResult instead")]
#[derive(Debug, Clone)]
pub struct SandboxHandle {
    pub id: String,
    pub status: String,
}

/// Old sandbox error type.
#[deprecated(note = "Use RuntimeError instead")]
#[derive(Debug)]
pub struct SandboxError;

#[allow(deprecated)]
impl std::fmt::Display for SandboxError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "SandboxError")
    }
}

#[allow(deprecated)]
impl std::error::Error for SandboxError {}

/// Old isolation backend trait.
#[deprecated(note = "Use RuntimeAdapter instead")]
#[async_trait]
pub trait IsolationProvider: Send + Sync {
    #[allow(deprecated)]
    async fn start(&self, spec: SandboxSpec) -> Result<SandboxHandle, SandboxError>;
    #[allow(deprecated)]
    async fn stop(&self, sandbox_id: &str) -> Result<(), SandboxError>;
}

/// Docker-based isolation (legacy stub, delegates to DockerRuntime for new code).
#[deprecated(note = "Use docker::DockerRuntime instead")]
pub struct DockerProvider;

#[allow(deprecated)]
impl DockerProvider {
    pub fn new() -> Self {
        Self
    }
}

#[allow(deprecated)]
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

// ---------------------------------------------------------------------------
// Helpers shared across runtimes
// ---------------------------------------------------------------------------

/// Environment variables safe to pass through to child processes.
pub const SAFE_ENV_VARS: &[&str] = &["PATH", "HOME", "TERM", "LANG", "USER", "SHELL", "TMPDIR"];

/// Truncate a byte buffer to at most `max_bytes`, respecting UTF-8 boundaries
/// when the content appears to be valid UTF-8.
pub fn truncate_output(buf: &[u8], max_bytes: usize) -> (Vec<u8>, bool) {
    if buf.len() <= max_bytes {
        return (buf.to_vec(), false);
    }
    let mut cutoff = max_bytes;
    // Try to respect UTF-8 char boundaries
    while cutoff > 0 && (buf[cutoff] & 0xC0) == 0x80 {
        cutoff -= 1;
    }
    let mut result = buf[..cutoff].to_vec();
    result.extend_from_slice(b"\n... [output truncated]");
    (result, true)
}
