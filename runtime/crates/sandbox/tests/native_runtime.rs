//! Integration tests for NativeRuntime.

#[cfg(not(target_os = "windows"))]
#[path = "../../../test_support/env_lock.rs"]
mod env_lock;
#[cfg(not(target_os = "windows"))]
#[path = "../../../test_support/temp_paths.rs"]
mod temp_paths;

use agentos_sandbox::native::NativeRuntime;
use agentos_sandbox::{ExecutionSpec, RuntimeAdapter};
#[cfg(not(target_os = "windows"))]
use std::fs;
#[cfg(not(target_os = "windows"))]
use std::path::PathBuf;
use std::time::Duration;

#[tokio::test]
async fn native_executes_echo() {
    let rt = NativeRuntime::new();
    if !rt.has_shell_access() {
        return; // skip on platforms without shell
    }

    let spec = ExecutionSpec {
        command: "echo hello-agentos".into(),
        ..ExecutionSpec::default()
    };
    let result = rt.execute(spec).await.expect("echo should succeed");
    assert_eq!(result.exit_code, 0);
    let stdout = String::from_utf8_lossy(&result.stdout);
    assert!(stdout.contains("hello-agentos"));
    assert!(!result.truncated);
}

#[tokio::test]
async fn native_captures_exit_code() {
    let rt = NativeRuntime::new();
    if !rt.has_shell_access() {
        return;
    }

    let spec = ExecutionSpec {
        command: "exit 42".into(),
        ..ExecutionSpec::default()
    };
    let result = rt.execute(spec).await.expect("exit should succeed");
    assert_eq!(result.exit_code, 42);
}

#[tokio::test]
async fn native_captures_stderr() {
    let rt = NativeRuntime::new();
    if !rt.has_shell_access() {
        return;
    }

    let spec = ExecutionSpec {
        command: "echo error-msg >&2".into(),
        ..ExecutionSpec::default()
    };
    let result = rt.execute(spec).await.expect("stderr test should succeed");
    let stderr = String::from_utf8_lossy(&result.stderr);
    assert!(stderr.contains("error-msg"));
}

#[tokio::test]
async fn native_timeout_kills_command() {
    let rt = NativeRuntime::new();
    if !rt.has_shell_access() {
        return;
    }

    let spec = ExecutionSpec {
        command: "sleep 60".into(),
        timeout: Duration::from_millis(200),
        ..ExecutionSpec::default()
    };
    let result = rt.execute(spec).await;
    assert!(result.is_err());
    let err = result.unwrap_err();
    assert!(err.to_string().contains("timed out"));
}

#[cfg(not(target_os = "windows"))]
#[tokio::test]
async fn native_timeout_cleans_up_shell_process() {
    let rt = NativeRuntime::new();
    if !rt.has_shell_access() {
        return;
    }

    let work_dir = unique_test_dir("timeout-cleanup");
    fs::create_dir_all(&work_dir).expect("temp dir should be created");
    let pid_file = work_dir.join("shell.pid");

    let spec = ExecutionSpec {
        command: "echo $$ > shell.pid; sleep 60".into(),
        working_dir: Some(work_dir.clone()),
        timeout: Duration::from_millis(200),
        ..ExecutionSpec::default()
    };

    let start = std::time::Instant::now();
    let result = rt.execute(spec).await;
    let elapsed = start.elapsed();
    assert!(result.is_err(), "timeout command should fail");

    let pid = wait_for_pid_file(&pid_file).await;
    let still_running = process_is_running(pid);
    if still_running {
        force_kill_process(pid);
    }

    fs::remove_file(&pid_file).ok();
    fs::remove_dir_all(&work_dir).ok();
    assert!(
        elapsed < Duration::from_secs(5),
        "timed out command took too long to return: {elapsed:?}"
    );
    assert!(
        !still_running,
        "timed out shell process {pid} was left running"
    );
}

#[tokio::test]
async fn native_output_truncation() {
    let rt = NativeRuntime::new();
    if !rt.has_shell_access() {
        return;
    }

    // Generate ~2000 bytes, limit to 100
    let spec = ExecutionSpec {
        command: "yes | head -200".into(),
        max_output_bytes: 100,
        ..ExecutionSpec::default()
    };
    let result = rt
        .execute(spec)
        .await
        .expect("truncation test should succeed");
    assert!(result.truncated);
    // Output should be approximately max_output_bytes
    assert!(result.stdout.len() < 200);
}

#[tokio::test]
async fn native_env_isolation() {
    let rt = NativeRuntime::new();
    if !rt.has_shell_access() {
        return;
    }

    // Set a "secret" env var in this process, verify it doesn't leak
    #[cfg(not(target_os = "windows"))]
    let _secret = env_lock::ScopedEnvVar::set("AGENTOS_TEST_SECRET", "super-secret-value")
        .expect("secret env var should be scoped");
    #[cfg(target_os = "windows")]
    std::env::set_var("AGENTOS_TEST_SECRET", "super-secret-value");

    let spec = ExecutionSpec {
        command: "env".into(),
        ..ExecutionSpec::default()
    };
    let result = rt.execute(spec).await.expect("env should succeed");
    let stdout = String::from_utf8_lossy(&result.stdout);
    assert!(
        !stdout.contains("AGENTOS_TEST_SECRET"),
        "secret env var leaked to child process"
    );
    assert!(stdout.contains("PATH="), "PATH should be passed through");
    #[cfg(target_os = "windows")]
    std::env::remove_var("AGENTOS_TEST_SECRET");
}

#[tokio::test]
async fn native_custom_env_vars() {
    let rt = NativeRuntime::new();
    if !rt.has_shell_access() {
        return;
    }

    let mut env = std::collections::HashMap::new();
    env.insert("MY_CUSTOM_VAR".into(), "custom_value".into());

    let spec = ExecutionSpec {
        command: "echo $MY_CUSTOM_VAR".into(),
        env,
        ..ExecutionSpec::default()
    };
    let result = rt.execute(spec).await.expect("custom env should succeed");
    let stdout = String::from_utf8_lossy(&result.stdout);
    assert!(stdout.contains("custom_value"));
}

#[tokio::test]
async fn native_working_dir() {
    let rt = NativeRuntime::new();
    if !rt.has_shell_access() {
        return;
    }

    let tmp = std::env::temp_dir();
    let spec = ExecutionSpec {
        command: "pwd".into(),
        working_dir: Some(tmp.clone()),
        ..ExecutionSpec::default()
    };
    let result = rt.execute(spec).await.expect("pwd should succeed");
    let stdout = String::from_utf8_lossy(&result.stdout);
    // The canonical path of /tmp may differ, but it should resolve to temp_dir
    assert!(!stdout.trim().is_empty());
}

#[test]
fn native_reports_name() {
    let rt = NativeRuntime::new();
    assert_eq!(rt.name(), "native");
}

#[test]
fn native_has_filesystem_access() {
    let rt = NativeRuntime::new();
    assert!(rt.has_filesystem_access());
}

#[test]
fn native_default_memory_budget_unlimited() {
    let rt = NativeRuntime::new();
    assert_eq!(rt.memory_budget(), 0);
}

#[test]
fn native_without_shell_denies_access() {
    let rt = NativeRuntime::new_without_shell();
    assert!(!rt.has_shell_access());
}

#[tokio::test]
async fn native_without_shell_returns_error() {
    let rt = NativeRuntime::new_without_shell();
    let spec = ExecutionSpec {
        command: "echo hello".into(),
        ..ExecutionSpec::default()
    };
    let result = rt.execute(spec).await;
    assert!(result.is_err());
    assert!(result.unwrap_err().to_string().contains("shell not found"));
}

#[cfg(not(target_os = "windows"))]
fn unique_test_dir(label: &str) -> PathBuf {
    temp_paths::unique_test_dir(&format!("native-runtime-{label}"))
}

#[cfg(not(target_os = "windows"))]
async fn wait_for_pid_file(path: &PathBuf) -> u32 {
    for _ in 0..20 {
        if let Ok(contents) = fs::read_to_string(path) {
            if let Ok(pid) = contents.trim().parse::<u32>() {
                return pid;
            }
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }
    panic!("pid file was not created: {}", path.display());
}

#[cfg(not(target_os = "windows"))]
fn process_is_running(pid: u32) -> bool {
    std::process::Command::new("kill")
        .arg("-0")
        .arg(pid.to_string())
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .status()
        .map(|status| status.success())
        .unwrap_or(false)
}

#[cfg(not(target_os = "windows"))]
fn force_kill_process(pid: u32) {
    let _ = std::process::Command::new("kill")
        .arg("-9")
        .arg(pid.to_string())
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .status();
}
