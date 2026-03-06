//! Factory function for creating runtime adapters from configuration.

use crate::docker::{DockerConfig, DockerRuntime};
use crate::native::NativeRuntime;
use crate::RuntimeAdapter;
use serde::{Deserialize, Serialize};

/// Top-level runtime configuration.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RuntimeConfig {
    /// Runtime kind: "native" or "docker".
    pub kind: String,
    /// Docker-specific config (used when kind = "docker").
    #[serde(default)]
    pub docker: DockerConfig,
}

impl Default for RuntimeConfig {
    fn default() -> Self {
        Self {
            kind: "native".into(),
            docker: DockerConfig::default(),
        }
    }
}

/// Create a runtime adapter from the given config.
pub fn create_runtime(config: &RuntimeConfig) -> Result<Box<dyn RuntimeAdapter>, String> {
    match config.kind.trim() {
        "native" => Ok(Box::new(NativeRuntime::new())),
        "docker" => Ok(Box::new(DockerRuntime::new(config.docker.clone()))),
        "" => Err("runtime kind cannot be empty. Supported: native, docker".into()),
        other => Err(format!(
            "unknown runtime kind '{other}'. Supported: native, docker"
        )),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn factory_creates_native() {
        let cfg = RuntimeConfig::default();
        let rt = create_runtime(&cfg).unwrap();
        assert_eq!(rt.name(), "native");
        assert!(rt.has_shell_access());
        assert!(rt.has_filesystem_access());
    }

    #[test]
    fn factory_creates_docker() {
        let cfg = RuntimeConfig {
            kind: "docker".into(),
            docker: DockerConfig::default(),
        };
        let rt = create_runtime(&cfg).unwrap();
        assert_eq!(rt.name(), "docker");
        assert!(rt.has_shell_access());
    }

    #[test]
    fn factory_rejects_empty() {
        let cfg = RuntimeConfig {
            kind: String::new(),
            ..RuntimeConfig::default()
        };
        match create_runtime(&cfg) {
            Err(err) => assert!(err.contains("cannot be empty")),
            Ok(_) => panic!("empty runtime kind should error"),
        }
    }

    #[test]
    fn factory_rejects_unknown() {
        let cfg = RuntimeConfig {
            kind: "serverless".into(),
            ..RuntimeConfig::default()
        };
        match create_runtime(&cfg) {
            Err(err) => assert!(err.contains("unknown runtime kind")),
            Ok(_) => panic!("unknown runtime kind should error"),
        }
    }
}
