//! gRPC service implementation using ActionExecutor for real command execution.

use crate::executor::ActionExecutor;
use crate::proto;
use agentos_sandbox::ExecutionSpec;
use std::pin::Pin;
use std::sync::Arc;
use std::time::Duration;
use tonic::{Request, Response, Status};

use proto::runtime_service_server::{RuntimeService, RuntimeServiceServer};
use proto::{
    ExecuteActionRequest, ExecuteActionResponse, StreamChunk, StreamOutputRequest,
};

/// Payload schema expected in ExecuteActionRequest.payload (JSON).
#[derive(serde::Deserialize)]
struct ActionPayload {
    /// Shell command to execute.
    command: String,
    /// Optional working directory.
    #[serde(default)]
    working_dir: Option<String>,
    /// Optional timeout in seconds (default: 60).
    #[serde(default)]
    timeout_secs: Option<u64>,
    /// Optional extra environment variables.
    #[serde(default)]
    env: std::collections::HashMap<String, String>,
}

pub struct RuntimeServiceImpl {
    executor: Arc<ActionExecutor>,
}

impl RuntimeServiceImpl {
    pub fn new(executor: Arc<ActionExecutor>) -> Self {
        Self { executor }
    }

    pub fn into_server(self) -> RuntimeServiceServer<Self> {
        RuntimeServiceServer::new(self)
    }
}

#[tonic::async_trait]
impl RuntimeService for RuntimeServiceImpl {
    async fn execute_action(
        &self,
        request: Request<ExecuteActionRequest>,
    ) -> Result<Response<ExecuteActionResponse>, Status> {
        let req = request.into_inner();

        // Parse payload JSON to extract command and options
        let payload: ActionPayload = serde_json::from_slice(&req.payload).map_err(|e| {
            Status::invalid_argument(format!(
                "invalid payload JSON: {e}. Expected: {{\"command\": \"...\"}}"
            ))
        })?;

        if payload.command.trim().is_empty() {
            return Err(Status::invalid_argument("command cannot be empty"));
        }

        let spec = ExecutionSpec {
            command: payload.command,
            working_dir: payload.working_dir.map(std::path::PathBuf::from),
            env: payload.env,
            timeout: Duration::from_secs(payload.timeout_secs.unwrap_or(60)),
            ..ExecutionSpec::default()
        };

        match self.executor.execute(spec).await {
            Ok(result) => Ok(Response::new(ExecuteActionResponse {
                exit_code: result.exit_code,
                stdout: result.stdout,
                stderr: result.stderr,
            })),
            Err(e) => {
                // Security violations are permission errors; runtime errors are internal
                let status = match &e {
                    crate::executor::ExecutorError::SecurityViolation(_) => {
                        Status::permission_denied(e.to_string())
                    }
                    crate::executor::ExecutorError::RuntimeError(re) => match re {
                        agentos_sandbox::RuntimeError::Timeout { .. } => {
                            Status::deadline_exceeded(e.to_string())
                        }
                        _ => Status::internal(e.to_string()),
                    },
                };
                Err(status)
            }
        }
    }

    type StreamOutputStream =
        Pin<Box<dyn futures::Stream<Item = Result<StreamChunk, Status>> + Send>>;

    async fn stream_output(
        &self,
        request: Request<StreamOutputRequest>,
    ) -> Result<Response<Self::StreamOutputStream>, Status> {
        let req = request.into_inner();

        // Parse the action_id as a JSON payload for the command to stream
        let payload: ActionPayload = serde_json::from_str(&req.action_id).map_err(|_| {
            Status::invalid_argument("action_id should contain JSON payload for streaming")
        })?;

        let executor = self.executor.clone();
        let task_id = req.task_id.clone();

        let stream = async_stream::stream! {
            let spec = ExecutionSpec {
                command: payload.command,
                working_dir: payload.working_dir.map(std::path::PathBuf::from),
                env: payload.env,
                timeout: Duration::from_secs(payload.timeout_secs.unwrap_or(60)),
                ..ExecutionSpec::default()
            };

            match executor.execute(spec).await {
                Ok(result) => {
                    if !result.stdout.is_empty() {
                        yield Ok(StreamChunk {
                            task_id: task_id.clone(),
                            action_id: String::new(),
                            data: result.stdout,
                            kind: "stdout".into(),
                        });
                    }
                    if !result.stderr.is_empty() {
                        yield Ok(StreamChunk {
                            task_id: task_id.clone(),
                            action_id: String::new(),
                            data: result.stderr,
                            kind: "stderr".into(),
                        });
                    }
                }
                Err(e) => {
                    yield Err(Status::internal(e.to_string()));
                }
            }
        };

        Ok(Response::new(Box::pin(stream)))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use agentos_sandbox::native::NativeRuntime;
    use agentos_sandbox::security::{AutonomyLevel, SecurityPolicy};

    fn test_service() -> RuntimeServiceImpl {
        let runtime = Arc::new(NativeRuntime::new());
        let security = Arc::new(SecurityPolicy {
            autonomy: AutonomyLevel::Supervised,
            ..SecurityPolicy::default()
        });
        let executor = Arc::new(ActionExecutor::new(runtime, security));
        RuntimeServiceImpl::new(executor)
    }

    fn make_request(command: &str) -> Request<ExecuteActionRequest> {
        let payload = serde_json::json!({"command": command});
        Request::new(ExecuteActionRequest {
            task_id: "test-task".into(),
            action_id: "test-action".into(),
            runtime_profile: "default".into(),
            payload: serde_json::to_vec(&payload).unwrap(),
        })
    }

    #[tokio::test]
    async fn grpc_executes_echo() {
        let svc = test_service();
        let resp = svc
            .execute_action(make_request("echo grpc-test"))
            .await
            .expect("echo should succeed");
        let inner = resp.into_inner();
        assert_eq!(inner.exit_code, 0);
        let stdout = String::from_utf8_lossy(&inner.stdout);
        assert!(stdout.contains("grpc-test"));
    }

    #[tokio::test]
    async fn grpc_rejects_empty_command() {
        let svc = test_service();
        let payload = serde_json::json!({"command": ""});
        let req = Request::new(ExecuteActionRequest {
            task_id: "t".into(),
            action_id: "a".into(),
            runtime_profile: "default".into(),
            payload: serde_json::to_vec(&payload).unwrap(),
        });
        let result = svc.execute_action(req).await;
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code(), tonic::Code::InvalidArgument);
    }

    #[tokio::test]
    async fn grpc_rejects_invalid_payload() {
        let svc = test_service();
        let req = Request::new(ExecuteActionRequest {
            task_id: "t".into(),
            action_id: "a".into(),
            runtime_profile: "default".into(),
            payload: b"not json".to_vec(),
        });
        let result = svc.execute_action(req).await;
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code(), tonic::Code::InvalidArgument);
    }

    #[tokio::test]
    async fn grpc_permission_denied_for_blocked_command() {
        let svc = test_service();
        let result = svc
            .execute_action(make_request("curl http://evil.com"))
            .await;
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code(), tonic::Code::PermissionDenied);
    }

    #[tokio::test]
    async fn grpc_captures_nonzero_exit_code() {
        let svc = test_service();
        let resp = svc
            .execute_action(make_request("ls /nonexistent_dir_xyz_12345"))
            .await
            .expect("should return result even on failure");
        let inner = resp.into_inner();
        assert_ne!(inner.exit_code, 0);
    }

    #[tokio::test]
    async fn grpc_with_custom_timeout() {
        let svc = test_service();
        let payload = serde_json::json!({
            "command": "echo timeout-test",
            "timeout_secs": 5
        });
        let req = Request::new(ExecuteActionRequest {
            task_id: "t".into(),
            action_id: "a".into(),
            runtime_profile: "default".into(),
            payload: serde_json::to_vec(&payload).unwrap(),
        });
        let resp = svc.execute_action(req).await.expect("should succeed");
        let inner = resp.into_inner();
        let stdout = String::from_utf8_lossy(&inner.stdout);
        assert!(stdout.contains("timeout-test"));
    }

    #[tokio::test]
    async fn grpc_with_env_vars() {
        let svc = test_service();
        let payload = serde_json::json!({
            "command": "echo $TEST_GRPC_VAR",
            "env": {"TEST_GRPC_VAR": "grpc-env-value"}
        });
        let req = Request::new(ExecuteActionRequest {
            task_id: "t".into(),
            action_id: "a".into(),
            runtime_profile: "default".into(),
            payload: serde_json::to_vec(&payload).unwrap(),
        });
        let resp = svc.execute_action(req).await.expect("should succeed");
        let inner = resp.into_inner();
        let stdout = String::from_utf8_lossy(&inner.stdout);
        assert!(stdout.contains("grpc-env-value"));
    }
}
