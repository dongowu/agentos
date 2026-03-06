//! Docker runtime -- executes commands inside lightweight containers.

use crate::{
    truncate_output, ExecutionResult, ExecutionSpec, RuntimeAdapter, RuntimeError, SAFE_ENV_VARS,
};
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use std::path::PathBuf;
use std::time::Instant;

/// Configuration for the Docker runtime.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DockerConfig {
    /// Container image (e.g. "ubuntu:22.04").
    pub image: String,
    /// Memory limit in megabytes (0 = no limit).
    pub memory_limit_mb: u64,
    /// CPU limit (e.g. 1.0 = one full core).
    pub cpu_limit: f64,
    /// Whether to mount the host workspace into the container.
    pub mount_workspace: bool,
    /// Whether to make the root filesystem read-only.
    pub read_only_rootfs: bool,
    /// Docker network mode ("none", "bridge", "host").
    pub network: String,
    /// Allowed host paths for workspace mounts. Empty = allow any.
    pub allowed_workspace_roots: Vec<PathBuf>,
}

impl Default for DockerConfig {
    fn default() -> Self {
        Self {
            image: "ubuntu:22.04".into(),
            memory_limit_mb: 512,
            cpu_limit: 1.0,
            mount_workspace: false,
            read_only_rootfs: false,
            network: "none".into(),
            allowed_workspace_roots: Vec::new(),
        }
    }
}

/// Executes commands inside Docker containers with resource limits.
pub struct DockerRuntime {
    config: DockerConfig,
}

impl DockerRuntime {
    pub fn new(config: DockerConfig) -> Self {
        Self { config }
    }

    /// Build the `docker run` command line from the spec.
    /// Exposed for testing without actually running Docker.
    pub fn build_command_args(&self, spec: &ExecutionSpec) -> Result<Vec<String>, RuntimeError> {
        let mut args = vec![
            "run".into(),
            "--rm".into(),
            "--init".into(),
        ];

        // Network isolation
        let network = self.config.network.trim();
        if !network.is_empty() {
            args.push("--network".into());
            args.push(network.to_string());
        }

        // Memory limit
        if self.config.memory_limit_mb > 0 {
            args.push("--memory".into());
            args.push(format!("{}m", self.config.memory_limit_mb));
        }

        // CPU limit
        if self.config.cpu_limit > 0.0 {
            args.push("--cpus".into());
            args.push(self.config.cpu_limit.to_string());
        }

        // Read-only rootfs
        if self.config.read_only_rootfs {
            args.push("--read-only".into());
        }

        // Workspace mount
        if self.config.mount_workspace {
            let workspace = spec
                .working_dir
                .as_ref()
                .ok_or_else(|| {
                    RuntimeError::Other(
                        "mount_workspace requires working_dir in ExecutionSpec".into(),
                    )
                })?;

            let resolved = workspace
                .canonicalize()
                .unwrap_or_else(|_| workspace.clone());

            if !resolved.is_absolute() {
                return Err(RuntimeError::Other(format!(
                    "Docker runtime requires absolute workspace path, got: {}",
                    resolved.display()
                )));
            }

            // Block root mount
            if resolved == std::path::Path::new("/") {
                return Err(RuntimeError::SecurityViolation(
                    "refusing to mount filesystem root (/) into container".into(),
                ));
            }

            // Check allowed roots
            if !self.config.allowed_workspace_roots.is_empty() {
                let allowed = self.config.allowed_workspace_roots.iter().any(|root| {
                    let root_resolved = root.canonicalize().unwrap_or_else(|_| root.clone());
                    resolved.starts_with(root_resolved)
                });
                if !allowed {
                    return Err(RuntimeError::SecurityViolation(format!(
                        "workspace path {} is not in allowed_workspace_roots",
                        resolved.display()
                    )));
                }
            }

            args.push("--volume".into());
            args.push(format!("{}:/workspace:rw", resolved.display()));
            args.push("--workdir".into());
            args.push("/workspace".into());
        }

        // Environment isolation: only pass safe vars
        for key in SAFE_ENV_VARS {
            if let Ok(val) = std::env::var(key) {
                args.push("--env".into());
                args.push(format!("{key}={val}"));
            }
        }

        // Caller-provided env vars
        for (k, v) in &spec.env {
            args.push("--env".into());
            args.push(format!("{k}={v}"));
        }

        // Image and command
        args.push(self.config.image.trim().to_string());
        args.push("sh".into());
        args.push("-c".into());
        args.push(spec.command.clone());

        Ok(args)
    }
}

#[async_trait]
impl RuntimeAdapter for DockerRuntime {
    fn name(&self) -> &str {
        "docker"
    }

    fn has_shell_access(&self) -> bool {
        true
    }

    fn has_filesystem_access(&self) -> bool {
        self.config.mount_workspace
    }

    fn memory_budget(&self) -> u64 {
        self.config.memory_limit_mb.saturating_mul(1024 * 1024)
    }

    async fn execute(&self, spec: ExecutionSpec) -> Result<ExecutionResult, RuntimeError> {
        let args = self.build_command_args(&spec)?;

        let mut cmd = tokio::process::Command::new("docker");
        cmd.args(&args);
        cmd.stdout(std::process::Stdio::piped());
        cmd.stderr(std::process::Stdio::piped());

        let start = Instant::now();

        let output = tokio::time::timeout(spec.timeout, cmd.output())
            .await
            .map_err(|_| RuntimeError::Timeout {
                elapsed: spec.timeout,
            })?
            .map_err(RuntimeError::IoError)?;

        let duration = start.elapsed();

        let (stdout, stdout_truncated) = truncate_output(&output.stdout, spec.max_output_bytes);
        let (stderr, stderr_truncated) = truncate_output(&output.stderr, spec.max_output_bytes);

        Ok(ExecutionResult {
            exit_code: output.status.code().unwrap_or(-1),
            stdout,
            stderr,
            duration,
            truncated: stdout_truncated || stderr_truncated,
        })
    }
}
