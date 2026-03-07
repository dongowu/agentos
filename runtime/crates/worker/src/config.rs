//! Worker configuration loaded from environment variables.

use agentos_sandbox::docker::DockerConfig;
use agentos_sandbox::factory::RuntimeConfig;
use agentos_sandbox::security::{AutonomyLevel, SecurityPolicy};
use std::path::PathBuf;

/// Worker-level configuration.
#[derive(Debug, Clone)]
pub struct WorkerConfig {
    /// Listen address for the gRPC server.
    pub listen_addr: String,
    /// Runtime configuration (native or docker).
    pub runtime: RuntimeConfig,
    /// Security policy for command execution.
    pub security: SecurityPolicy,
    /// Unique identifier for this worker (used in registration).
    pub worker_id: String,
    /// Address of the control plane WorkerRegistry gRPC service.
    pub control_plane_addr: Option<String>,
    /// Interval between heartbeats, in seconds.
    pub heartbeat_interval_secs: u64,
    /// Maximum number of concurrent tasks this worker will accept.
    pub max_concurrent_tasks: u32,
}

impl Default for WorkerConfig {
    fn default() -> Self {
        Self {
            listen_addr: "127.0.0.1:50051".into(),
            runtime: RuntimeConfig::default(),
            security: SecurityPolicy::default(),
            worker_id: generate_worker_id(),
            control_plane_addr: None,
            heartbeat_interval_secs: 10,
            max_concurrent_tasks: 4,
        }
    }
}

/// Generate a default worker id: hostname-random_hex.
fn generate_worker_id() -> String {
    let host = hostname();
    let rand_bytes: [u8; 4] = rand_bytes_4();
    let hex: String = rand_bytes.iter().map(|b| format!("{b:02x}")).collect();
    format!("{host}-{hex}")
}

/// Best-effort hostname; falls back to "worker".
fn hostname() -> String {
    std::env::var("HOSTNAME")
        .or_else(|_| {
            std::process::Command::new("hostname")
                .output()
                .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        })
        .unwrap_or_else(|_| "worker".into())
}

/// Grab 4 pseudo-random bytes without pulling in the `rand` crate.
fn rand_bytes_4() -> [u8; 4] {
    let t = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    let pid = std::process::id() as u128;
    let mix = t.wrapping_mul(6364136223846793005).wrapping_add(pid);
    (mix as u32).to_le_bytes()
}

impl WorkerConfig {
    /// Load configuration from environment variables.
    ///
    /// Supported variables:
    /// - `AGENTOS_LISTEN_ADDR` -- gRPC listen address (default: `127.0.0.1:50051`)
    /// - `AGENTOS_RUNTIME` -- `native` or `docker` (default: `native`)
    /// - `AGENTOS_DOCKER_IMAGE` -- Docker image (default: `ubuntu:22.04`)
    /// - `AGENTOS_DOCKER_MEMORY_MB` -- Docker memory limit in MB (default: `512`)
    /// - `AGENTOS_DOCKER_CPU_LIMIT` -- Docker CPU limit (default: `1.0`)
    /// - `AGENTOS_DOCKER_NETWORK` -- Docker network mode (default: `none`)
    /// - `AGENTOS_DOCKER_MOUNT_WORKSPACE` -- Mount host workspace (default: `false`)
    /// - `AGENTOS_DOCKER_READ_ONLY` -- Read-only rootfs (default: `false`)
    /// - `AGENTOS_SECURITY_LEVEL` -- `supervised`, `semi`, or `autonomous` (default: `supervised`)
    /// - `AGENTOS_MAX_ACTIONS_PER_HOUR` -- Rate limit (default: `120`)
    /// - `AGENTOS_MAX_OUTPUT_BYTES` -- Output truncation limit (default: `1048576`)
    /// - `AGENTOS_FORBIDDEN_PATHS` -- Comma-separated forbidden paths
    /// - `AGENTOS_WORKER_ID` -- Unique worker id (default: hostname-random)
    /// - `AGENTOS_CONTROL_PLANE_ADDR` -- Control plane address for registration
    /// - `AGENTOS_HEARTBEAT_INTERVAL_SECS` -- Heartbeat interval (default: `10`)
    /// - `AGENTOS_MAX_CONCURRENT_TASKS` -- Max concurrent tasks (default: `4`)
    pub fn from_env() -> Self {
        let listen_addr =
            std::env::var("AGENTOS_LISTEN_ADDR").unwrap_or_else(|_| "127.0.0.1:50051".into());

        let worker_id = std::env::var("AGENTOS_WORKER_ID").unwrap_or_else(|_| generate_worker_id());

        let control_plane_addr = std::env::var("AGENTOS_CONTROL_PLANE_ADDR").ok();

        let heartbeat_interval_secs = std::env::var("AGENTOS_HEARTBEAT_INTERVAL_SECS")
            .ok()
            .and_then(|v| v.parse().ok())
            .unwrap_or(10);

        let max_concurrent_tasks = std::env::var("AGENTOS_MAX_CONCURRENT_TASKS")
            .ok()
            .and_then(|v| v.parse().ok())
            .unwrap_or(4);

        let runtime_kind = std::env::var("AGENTOS_RUNTIME").unwrap_or_else(|_| "native".into());

        let docker = DockerConfig {
            image: std::env::var("AGENTOS_DOCKER_IMAGE").unwrap_or_else(|_| "ubuntu:22.04".into()),
            memory_limit_mb: std::env::var("AGENTOS_DOCKER_MEMORY_MB")
                .ok()
                .and_then(|v| v.parse().ok())
                .unwrap_or(512),
            cpu_limit: std::env::var("AGENTOS_DOCKER_CPU_LIMIT")
                .ok()
                .and_then(|v| v.parse().ok())
                .unwrap_or(1.0),
            network: std::env::var("AGENTOS_DOCKER_NETWORK").unwrap_or_else(|_| "none".into()),
            mount_workspace: std::env::var("AGENTOS_DOCKER_MOUNT_WORKSPACE")
                .map(|v| v == "true" || v == "1")
                .unwrap_or(false),
            read_only_rootfs: std::env::var("AGENTOS_DOCKER_READ_ONLY")
                .map(|v| v == "true" || v == "1")
                .unwrap_or(false),
            allowed_workspace_roots: Vec::new(),
        };

        let autonomy = match std::env::var("AGENTOS_SECURITY_LEVEL")
            .unwrap_or_else(|_| "supervised".into())
            .to_lowercase()
            .as_str()
        {
            "autonomous" | "auto" => AutonomyLevel::Autonomous,
            "semi" | "semiautonomous" | "semi_autonomous" => AutonomyLevel::SemiAutonomous,
            _ => AutonomyLevel::Supervised,
        };

        let max_actions_per_hour = std::env::var("AGENTOS_MAX_ACTIONS_PER_HOUR")
            .ok()
            .and_then(|v| v.parse().ok())
            .unwrap_or(120);

        let max_output_bytes = std::env::var("AGENTOS_MAX_OUTPUT_BYTES")
            .ok()
            .and_then(|v| v.parse().ok())
            .unwrap_or(1_048_576);

        let forbidden_paths = std::env::var("AGENTOS_FORBIDDEN_PATHS")
            .map(|v| {
                v.split(',')
                    .map(|s| PathBuf::from(s.trim()))
                    .filter(|p| !p.as_os_str().is_empty())
                    .collect()
            })
            .unwrap_or_else(|_| SecurityPolicy::default().forbidden_paths);

        let security = SecurityPolicy {
            autonomy,
            max_actions_per_hour,
            max_output_bytes,
            forbidden_paths,
            ..SecurityPolicy::default()
        };

        Self {
            listen_addr,
            runtime: RuntimeConfig {
                kind: runtime_kind,
                docker,
            },
            security,
            worker_id,
            control_plane_addr,
            heartbeat_interval_secs,
            max_concurrent_tasks,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_config_is_native_supervised() {
        let cfg = WorkerConfig::default();
        assert_eq!(cfg.runtime.kind, "native");
        assert_eq!(cfg.security.autonomy, AutonomyLevel::Supervised);
        assert_eq!(cfg.listen_addr, "127.0.0.1:50051");
        assert!(!cfg.worker_id.is_empty());
        assert_eq!(cfg.control_plane_addr, None);
        assert_eq!(cfg.heartbeat_interval_secs, 10);
        assert_eq!(cfg.max_concurrent_tasks, 4);
    }

    #[test]
    fn from_env_defaults_without_vars() {
        // With no env vars set, should produce sane defaults
        let cfg = WorkerConfig::from_env();
        assert!(!cfg.listen_addr.is_empty());
        assert!(!cfg.runtime.kind.is_empty());
        assert!(!cfg.worker_id.is_empty());
        assert_eq!(cfg.heartbeat_interval_secs, 10);
        assert_eq!(cfg.max_concurrent_tasks, 4);
    }

    #[test]
    fn worker_id_contains_hostname_and_hex() {
        let id = generate_worker_id();
        assert!(id.contains('-'), "worker_id should contain a dash: {id}");
        // The hex suffix should be 8 chars (4 bytes)
        let suffix = id.rsplit('-').next().unwrap();
        assert_eq!(suffix.len(), 8, "hex suffix should be 8 chars: {suffix}");
        assert!(suffix.chars().all(|c| c.is_ascii_hexdigit()));
    }
}
