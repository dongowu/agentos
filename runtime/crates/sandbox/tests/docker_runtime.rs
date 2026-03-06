//! Integration tests for DockerRuntime (command construction only, no real Docker).

use agentos_sandbox::docker::{DockerConfig, DockerRuntime};
use agentos_sandbox::{ExecutionSpec, RuntimeAdapter};
use std::path::PathBuf;

#[test]
fn docker_reports_name() {
    let rt = DockerRuntime::new(DockerConfig::default());
    assert_eq!(rt.name(), "docker");
}

#[test]
fn docker_has_shell_access() {
    let rt = DockerRuntime::new(DockerConfig::default());
    assert!(rt.has_shell_access());
}

#[test]
fn docker_filesystem_access_follows_config() {
    let no_mount = DockerRuntime::new(DockerConfig {
        mount_workspace: false,
        ..DockerConfig::default()
    });
    assert!(!no_mount.has_filesystem_access());

    let with_mount = DockerRuntime::new(DockerConfig {
        mount_workspace: true,
        ..DockerConfig::default()
    });
    assert!(with_mount.has_filesystem_access());
}

#[test]
fn docker_memory_budget_from_config() {
    let rt = DockerRuntime::new(DockerConfig {
        memory_limit_mb: 256,
        ..DockerConfig::default()
    });
    assert_eq!(rt.memory_budget(), 256 * 1024 * 1024);
}

#[test]
fn docker_memory_budget_zero_when_no_limit() {
    let rt = DockerRuntime::new(DockerConfig {
        memory_limit_mb: 0,
        ..DockerConfig::default()
    });
    assert_eq!(rt.memory_budget(), 0);
}

#[test]
fn docker_command_includes_resource_limits() {
    let rt = DockerRuntime::new(DockerConfig {
        image: "alpine:3.20".into(),
        memory_limit_mb: 128,
        cpu_limit: 1.5,
        network: "none".into(),
        ..DockerConfig::default()
    });

    let spec = ExecutionSpec {
        command: "echo hello".into(),
        ..ExecutionSpec::default()
    };
    let args = rt.build_command_args(&spec).unwrap();
    let joined = args.join(" ");

    assert!(joined.contains("--memory 128m"));
    assert!(joined.contains("--cpus 1.5"));
    assert!(joined.contains("--network none"));
    assert!(joined.contains("alpine:3.20"));
    assert!(joined.contains("echo hello"));
}

#[test]
fn docker_command_includes_read_only() {
    let rt = DockerRuntime::new(DockerConfig {
        read_only_rootfs: true,
        ..DockerConfig::default()
    });

    let spec = ExecutionSpec {
        command: "ls".into(),
        ..ExecutionSpec::default()
    };
    let args = rt.build_command_args(&spec).unwrap();
    assert!(args.contains(&"--read-only".to_string()));
}

#[test]
fn docker_command_includes_workspace_mount() {
    let rt = DockerRuntime::new(DockerConfig {
        mount_workspace: true,
        ..DockerConfig::default()
    });

    let spec = ExecutionSpec {
        command: "ls".into(),
        working_dir: Some(std::env::temp_dir()),
        ..ExecutionSpec::default()
    };
    let args = rt.build_command_args(&spec).unwrap();
    let joined = args.join(" ");
    assert!(joined.contains("--volume"));
    assert!(joined.contains("/workspace:rw"));
    assert!(joined.contains("--workdir /workspace"));
}

#[test]
fn docker_command_no_memory_flag_when_zero() {
    let rt = DockerRuntime::new(DockerConfig {
        memory_limit_mb: 0,
        ..DockerConfig::default()
    });

    let spec = ExecutionSpec {
        command: "ls".into(),
        ..ExecutionSpec::default()
    };
    let args = rt.build_command_args(&spec).unwrap();
    assert!(!args.contains(&"--memory".to_string()));
}

#[test]
fn docker_command_includes_rm_and_init() {
    let rt = DockerRuntime::new(DockerConfig::default());
    let spec = ExecutionSpec {
        command: "ls".into(),
        ..ExecutionSpec::default()
    };
    let args = rt.build_command_args(&spec).unwrap();
    assert!(args.contains(&"--rm".to_string()));
    assert!(args.contains(&"--init".to_string()));
}

#[cfg(unix)]
#[test]
fn docker_refuses_root_mount() {
    let rt = DockerRuntime::new(DockerConfig {
        mount_workspace: true,
        ..DockerConfig::default()
    });

    let spec = ExecutionSpec {
        command: "ls".into(),
        working_dir: Some(PathBuf::from("/")),
        ..ExecutionSpec::default()
    };
    let result = rt.build_command_args(&spec);
    assert!(result.is_err());
    let err = result.unwrap_err().to_string();
    assert!(err.contains("root"));
}

#[test]
fn docker_workspace_allowlist_blocks_outside_paths() {
    let rt = DockerRuntime::new(DockerConfig {
        mount_workspace: true,
        allowed_workspace_roots: vec![PathBuf::from("/tmp/allowed")],
        ..DockerConfig::default()
    });

    let spec = ExecutionSpec {
        command: "ls".into(),
        working_dir: Some(PathBuf::from("/tmp/blocked")),
        ..ExecutionSpec::default()
    };
    let result = rt.build_command_args(&spec);
    assert!(result.is_err());
    let err = result.unwrap_err().to_string();
    assert!(err.contains("allowed_workspace_roots"));
}

#[test]
fn docker_mount_requires_working_dir() {
    let rt = DockerRuntime::new(DockerConfig {
        mount_workspace: true,
        ..DockerConfig::default()
    });

    let spec = ExecutionSpec {
        command: "ls".into(),
        working_dir: None,
        ..ExecutionSpec::default()
    };
    let result = rt.build_command_args(&spec);
    assert!(result.is_err());
    assert!(result
        .unwrap_err()
        .to_string()
        .contains("mount_workspace requires working_dir"));
}

#[test]
fn docker_passes_custom_env_vars() {
    let rt = DockerRuntime::new(DockerConfig::default());

    let mut env = std::collections::HashMap::new();
    env.insert("MY_VAR".into(), "my_value".into());

    let spec = ExecutionSpec {
        command: "echo test".into(),
        env,
        ..ExecutionSpec::default()
    };
    let args = rt.build_command_args(&spec).unwrap();
    let joined = args.join(" ");
    assert!(joined.contains("--env MY_VAR=my_value"));
}
