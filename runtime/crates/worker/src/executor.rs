//! ActionExecutor -- validates, executes, and post-processes commands.

use agentos_sandbox::security::{SecurityError, SecurityPolicy};
use agentos_sandbox::{ExecutionResult, ExecutionSpec, RuntimeAdapter, RuntimeError};
use std::sync::Arc;

/// Errors from the executor layer.
#[derive(Debug)]
pub enum ExecutorError {
    /// Command was rejected by security policy.
    SecurityViolation(SecurityError),
    /// Runtime failed to execute the command.
    RuntimeError(RuntimeError),
}

impl std::fmt::Display for ExecutorError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ExecutorError::SecurityViolation(e) => write!(f, "security: {e}"),
            ExecutorError::RuntimeError(e) => write!(f, "runtime: {e}"),
        }
    }
}

impl std::error::Error for ExecutorError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            ExecutorError::SecurityViolation(e) => Some(e),
            ExecutorError::RuntimeError(e) => Some(e),
        }
    }
}

impl From<SecurityError> for ExecutorError {
    fn from(e: SecurityError) -> Self {
        ExecutorError::SecurityViolation(e)
    }
}

impl From<RuntimeError> for ExecutorError {
    fn from(e: RuntimeError) -> Self {
        ExecutorError::RuntimeError(e)
    }
}

/// Wraps a `RuntimeAdapter` and `SecurityPolicy` to provide validated command execution.
pub struct ActionExecutor {
    runtime: Arc<dyn RuntimeAdapter>,
    security: Arc<SecurityPolicy>,
}

impl ActionExecutor {
    pub fn new(runtime: Arc<dyn RuntimeAdapter>, security: Arc<SecurityPolicy>) -> Self {
        Self { runtime, security }
    }

    /// Execute a single command after security validation and with secret redaction.
    pub async fn execute(&self, spec: ExecutionSpec) -> Result<ExecutionResult, ExecutorError> {
        // 1. Validate command against security policy
        self.security.validate_command(&spec.command)?;

        // 2. Enforce policy output limit if spec's is larger
        let spec = ExecutionSpec {
            max_output_bytes: spec.max_output_bytes.min(self.security.max_output_bytes),
            ..spec
        };

        // 3. Execute via runtime adapter
        let mut result = self.runtime.execute(spec).await?;

        // 4. Redact secrets from output
        let stdout_str = String::from_utf8_lossy(&result.stdout);
        let redacted_stdout = self.security.redact_secrets(&stdout_str);
        result.stdout = redacted_stdout.into_bytes();

        let stderr_str = String::from_utf8_lossy(&result.stderr);
        let redacted_stderr = self.security.redact_secrets(&stderr_str);
        result.stderr = redacted_stderr.into_bytes();

        Ok(result)
    }

    /// Execute multiple commands in parallel, returning results in order.
    pub async fn execute_parallel(
        &self,
        specs: Vec<ExecutionSpec>,
    ) -> Vec<Result<ExecutionResult, ExecutorError>> {
        let futures: Vec<_> = specs.into_iter().map(|spec| self.execute(spec)).collect();
        futures::future::join_all(futures).await
    }

    /// Access the underlying runtime adapter.
    pub fn runtime(&self) -> &dyn RuntimeAdapter {
        self.runtime.as_ref()
    }

    /// Access the security policy.
    pub fn security(&self) -> &SecurityPolicy {
        &self.security
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use agentos_sandbox::native::NativeRuntime;
    use agentos_sandbox::security::AutonomyLevel;

    fn test_executor(autonomy: AutonomyLevel) -> ActionExecutor {
        let runtime = Arc::new(NativeRuntime::new());
        let security = Arc::new(SecurityPolicy {
            autonomy,
            ..SecurityPolicy::default()
        });
        ActionExecutor::new(runtime, security)
    }

    #[tokio::test]
    async fn executor_runs_allowed_command() {
        let exec = test_executor(AutonomyLevel::Supervised);
        let spec = ExecutionSpec {
            command: "echo executor-test".into(),
            ..ExecutionSpec::default()
        };
        let result = exec.execute(spec).await.expect("echo should succeed");
        assert_eq!(result.exit_code, 0);
        let stdout = String::from_utf8_lossy(&result.stdout);
        assert!(stdout.contains("executor-test"));
    }

    #[tokio::test]
    async fn executor_blocks_disallowed_command() {
        let exec = test_executor(AutonomyLevel::Supervised);
        let spec = ExecutionSpec {
            command: "curl http://example.com".into(),
            ..ExecutionSpec::default()
        };
        let result = exec.execute(spec).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("security"));
    }

    #[tokio::test]
    async fn executor_blocks_blacklisted_even_autonomous() {
        let exec = test_executor(AutonomyLevel::Autonomous);
        let spec = ExecutionSpec {
            command: "rm -rf /".into(),
            ..ExecutionSpec::default()
        };
        let result = exec.execute(spec).await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn executor_redacts_secrets_from_output() {
        let exec = test_executor(AutonomyLevel::Supervised);
        let spec = ExecutionSpec {
            command: "echo token=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn".into(),
            ..ExecutionSpec::default()
        };
        let result = exec.execute(spec).await.expect("echo should succeed");
        let stdout = String::from_utf8_lossy(&result.stdout);
        assert!(!stdout.contains("ghp_ABCDEFGHIJ"));
        assert!(stdout.contains("[REDACTED]"));
    }

    #[tokio::test]
    async fn executor_parallel_runs_multiple() {
        let exec = test_executor(AutonomyLevel::Supervised);
        let specs = vec![
            ExecutionSpec {
                command: "echo first".into(),
                ..ExecutionSpec::default()
            },
            ExecutionSpec {
                command: "echo second".into(),
                ..ExecutionSpec::default()
            },
        ];
        let results = exec.execute_parallel(specs).await;
        assert_eq!(results.len(), 2);
        for r in &results {
            assert!(r.is_ok());
        }
        let first = String::from_utf8_lossy(&results[0].as_ref().unwrap().stdout);
        let second = String::from_utf8_lossy(&results[1].as_ref().unwrap().stdout);
        assert!(first.contains("first"));
        assert!(second.contains("second"));
    }

    #[tokio::test]
    async fn executor_enforces_policy_output_limit() {
        let runtime = Arc::new(NativeRuntime::new());
        let security = Arc::new(SecurityPolicy {
            autonomy: AutonomyLevel::Supervised,
            max_output_bytes: 50,
            ..SecurityPolicy::default()
        });
        let exec = ActionExecutor::new(runtime, security);

        // Request a large max but policy caps it
        let spec = ExecutionSpec {
            command: "echo hello".into(),
            max_output_bytes: 10_000_000,
            ..ExecutionSpec::default()
        };
        // Should work -- the point is that policy limit is respected
        let result = exec.execute(spec).await.expect("should succeed");
        assert_eq!(result.exit_code, 0);
    }

    #[test]
    fn executor_exposes_runtime_name() {
        let exec = test_executor(AutonomyLevel::Supervised);
        assert_eq!(exec.runtime().name(), "native");
    }

    #[test]
    fn executor_exposes_security_policy() {
        let exec = test_executor(AutonomyLevel::Autonomous);
        assert_eq!(exec.security().autonomy, AutonomyLevel::Autonomous);
    }
}
