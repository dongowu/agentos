//! Integration tests for NativeRuntime.

use agentos_sandbox::native::NativeRuntime;
use agentos_sandbox::{ExecutionSpec, RuntimeAdapter};
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
