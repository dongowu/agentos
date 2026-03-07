//! gRPC service implementation using ActionExecutor for real command execution.

use crate::executor::{ActionExecutor, ExecutorError};
use crate::proto;
use agentos_sandbox::docker::DockerRuntime;
use agentos_sandbox::{ExecutionSpec, SAFE_ENV_VARS};
use std::path::PathBuf;
use std::pin::Pin;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::io::AsyncReadExt;
use tonic::{Request, Response, Status};

use proto::runtime_service_server::{RuntimeService, RuntimeServiceServer};
use proto::{ExecuteActionRequest, ExecuteActionResponse, StreamChunk, StreamOutputRequest};

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

fn payload_from_bytes(bytes: &[u8]) -> Result<ActionPayload, Status> {
    let payload: ActionPayload = serde_json::from_slice(bytes).map_err(|e| {
        Status::invalid_argument(format!(
            "invalid payload JSON: {e}. Expected: {{\"command\": \"...\"}}"
        ))
    })?;
    if payload.command.trim().is_empty() {
        return Err(Status::invalid_argument("command cannot be empty"));
    }
    Ok(payload)
}

fn stream_payload_from_request(req: &StreamOutputRequest) -> Result<ActionPayload, Status> {
    if !req.payload.is_empty() {
        return payload_from_bytes(&req.payload);
    }
    let payload: ActionPayload = serde_json::from_str(&req.action_id).map_err(|_| {
        Status::invalid_argument(
            "stream payload required: send JSON in payload or legacy action_id field",
        )
    })?;
    if payload.command.trim().is_empty() {
        return Err(Status::invalid_argument("command cannot be empty"));
    }
    Ok(payload)
}

fn payload_to_spec(payload: ActionPayload, max_output_bytes: usize) -> ExecutionSpec {
    ExecutionSpec {
        command: payload.command,
        working_dir: payload.working_dir.map(std::path::PathBuf::from),
        env: payload.env,
        timeout: Duration::from_secs(payload.timeout_secs.unwrap_or(60)),
        max_output_bytes,
    }
}

fn executor_error_to_status(error: ExecutorError) -> Status {
    match &error {
        ExecutorError::SecurityViolation(_) => Status::permission_denied(error.to_string()),
        ExecutorError::RuntimeError(re) => match re {
            agentos_sandbox::RuntimeError::Timeout { .. } => {
                Status::deadline_exceeded(error.to_string())
            }
            _ => Status::internal(error.to_string()),
        },
    }
}

fn which_lookup(name: &str) -> Option<PathBuf> {
    let path_var = std::env::var_os("PATH")?;
    for dir in std::env::split_paths(&path_var) {
        let candidate = dir.join(name);
        if candidate.is_file() {
            return Some(candidate);
        }
    }
    None
}

fn build_native_command(spec: &ExecutionSpec) -> Result<tokio::process::Command, Status> {
    #[cfg(target_os = "windows")]
    let shell = which_lookup("bash")
        .or_else(|| which_lookup("sh"))
        .or_else(|| which_lookup("cmd"))
        .or_else(|| std::env::var_os("COMSPEC").map(PathBuf::from))
        .ok_or_else(|| Status::internal("no usable shell found for streaming"))?;

    #[cfg(not(target_os = "windows"))]
    let shell = which_lookup("sh")
        .or_else(|| which_lookup("bash"))
        .ok_or_else(|| Status::internal("no usable shell found for streaming"))?;

    let mut cmd = tokio::process::Command::new(shell);
    #[cfg(target_os = "windows")]
    {
        cmd.arg("/C").arg(&spec.command);
    }
    #[cfg(not(target_os = "windows"))]
    {
        cmd.arg("-c").arg(&spec.command);
    }

    if let Some(ref dir) = spec.working_dir {
        cmd.current_dir(dir);
    }
    cmd.env_clear();
    for key in SAFE_ENV_VARS {
        if let Ok(val) = std::env::var(key) {
            cmd.env(key, val);
        }
    }
    for (key, value) in &spec.env {
        cmd.env(key, value);
    }
    cmd.stdout(std::process::Stdio::piped());
    cmd.stderr(std::process::Stdio::piped());
    Ok(cmd)
}

fn build_docker_command(
    spec: &ExecutionSpec,
    runtime: &DockerRuntime,
) -> Result<tokio::process::Command, Status> {
    let args = runtime
        .build_command_args(spec)
        .map_err(|error| executor_error_to_status(ExecutorError::RuntimeError(error)))?;

    let mut cmd = tokio::process::Command::new("docker");
    cmd.args(&args);
    cmd.stdout(std::process::Stdio::piped());
    cmd.stderr(std::process::Stdio::piped());
    Ok(cmd)
}

fn build_streaming_command(
    executor: &ActionExecutor,
    spec: &ExecutionSpec,
) -> Result<Option<tokio::process::Command>, Status> {
    let runtime = executor.runtime();
    match runtime.name() {
        "native" => build_native_command(spec).map(Some),
        "docker" => {
            let docker = runtime
                .as_any()
                .downcast_ref::<DockerRuntime>()
                .ok_or_else(|| Status::internal("docker runtime downcast failed"))?;
            build_docker_command(spec, docker).map(Some)
        }
        _ => Ok(None),
    }
}

async fn pump_reader<R>(
    mut reader: R,
    kind: &'static str,
    task_id: String,
    action_id: String,
    security: agentos_sandbox::security::SecurityPolicy,
    limit: usize,
    sent: Arc<AtomicUsize>,
    truncated_sent: Arc<AtomicBool>,
    tx: tokio::sync::mpsc::Sender<Result<StreamChunk, Status>>,
) where
    R: tokio::io::AsyncRead + Unpin + Send + 'static,
{
    let mut buf = [0_u8; 4096];
    loop {
        match reader.read(&mut buf).await {
            Ok(0) => return,
            Ok(read) => {
                let already_sent = sent.fetch_add(read, Ordering::SeqCst);
                if already_sent >= limit {
                    if !truncated_sent.swap(true, Ordering::SeqCst) {
                        let _ = tx
                            .send(Ok(StreamChunk {
                                task_id: task_id.clone(),
                                action_id: action_id.clone(),
                                data: b"\n... [output truncated]".to_vec(),
                                kind: kind.into(),
                            }))
                            .await;
                    }
                    return;
                }
                let remaining = limit.saturating_sub(already_sent);
                let allowed = remaining.min(read);
                let text = String::from_utf8_lossy(&buf[..allowed]);
                let redacted = security.redact_secrets(&text);
                if !redacted.is_empty() {
                    let _ = tx
                        .send(Ok(StreamChunk {
                            task_id: task_id.clone(),
                            action_id: action_id.clone(),
                            data: redacted.into_bytes(),
                            kind: kind.into(),
                        }))
                        .await;
                }
                if allowed < read && !truncated_sent.swap(true, Ordering::SeqCst) {
                    let _ = tx
                        .send(Ok(StreamChunk {
                            task_id: task_id.clone(),
                            action_id: action_id.clone(),
                            data: b"\n... [output truncated]".to_vec(),
                            kind: kind.into(),
                        }))
                        .await;
                    return;
                }
            }
            Err(error) => {
                let _ = tx.send(Err(Status::internal(error.to_string()))).await;
                return;
            }
        }
    }
}

#[tonic::async_trait]
impl RuntimeService for RuntimeServiceImpl {
    async fn execute_action(
        &self,
        request: Request<ExecuteActionRequest>,
    ) -> Result<Response<ExecuteActionResponse>, Status> {
        let req = request.into_inner();
        let payload = payload_from_bytes(&req.payload)?;
        let spec = payload_to_spec(payload, self.executor.security().max_output_bytes);

        match self.executor.execute(spec).await {
            Ok(result) => Ok(Response::new(ExecuteActionResponse {
                exit_code: result.exit_code,
                stdout: result.stdout,
                stderr: result.stderr,
            })),
            Err(error) => Err(executor_error_to_status(error)),
        }
    }

    type StreamOutputStream =
        Pin<Box<dyn futures::Stream<Item = Result<StreamChunk, Status>> + Send>>;

    async fn stream_output(
        &self,
        request: Request<StreamOutputRequest>,
    ) -> Result<Response<Self::StreamOutputStream>, Status> {
        let req = request.into_inner();
        let payload = stream_payload_from_request(&req)?;

        let executor = self.executor.clone();
        let task_id = req.task_id.clone();
        let action_id = req.action_id.clone();
        let max_output_bytes = executor.security().max_output_bytes;
        let spec = payload_to_spec(payload, max_output_bytes);

        let stream = async_stream::stream! {
            if let Err(error) = executor.security().validate_command(&spec.command) {
                yield Err(Status::permission_denied(format!("security: {error}")));
                return;
            }

            let mut command = match build_streaming_command(executor.as_ref(), &spec) {
                Ok(Some(command)) => command,
                Ok(None) => {
                    match executor.execute(spec).await {
                        Ok(result) => {
                            if !result.stdout.is_empty() {
                                yield Ok(StreamChunk {
                                    task_id: task_id.clone(),
                                    action_id: action_id.clone(),
                                    data: result.stdout,
                                    kind: "stdout".into(),
                                });
                            }
                            if !result.stderr.is_empty() {
                                yield Ok(StreamChunk {
                                    task_id: task_id.clone(),
                                    action_id: action_id.clone(),
                                    data: result.stderr,
                                    kind: "stderr".into(),
                                });
                            }
                            yield Ok(StreamChunk {
                                task_id: task_id.clone(),
                                action_id: action_id.clone(),
                                data: result.exit_code.to_string().into_bytes(),
                                kind: "exit".into(),
                            });
                        }
                        Err(error) => {
                            yield Err(executor_error_to_status(error));
                        }
                    }
                    return;
                }
                Err(error) => {
                    yield Err(error);
                    return;
                }
            };
            let runtime_name = executor.runtime().name().to_string();
            let mut child = match command.spawn() {
                Ok(child) => child,
                Err(error) => {
                    if runtime_name == "docker" && error.kind() == std::io::ErrorKind::NotFound {
                        yield Err(Status::internal("docker binary not found on PATH. Install Docker to use the Docker runtime."));
                    } else {
                        yield Err(Status::internal(error.to_string()));
                    }
                    return;
                }
            };
            let stdout = match child.stdout.take() {
                Some(stdout) => stdout,
                None => {
                    yield Err(Status::internal("missing stdout pipe"));
                    return;
                }
            };
            let stderr = match child.stderr.take() {
                Some(stderr) => stderr,
                None => {
                    yield Err(Status::internal("missing stderr pipe"));
                    return;
                }
            };

            let security = executor.security().clone();
            let sent = Arc::new(AtomicUsize::new(0));
            let truncated_sent = Arc::new(AtomicBool::new(false));
            let (tx, mut rx) = tokio::sync::mpsc::channel::<Result<StreamChunk, Status>>(32);
            let stdout_task = tokio::spawn(pump_reader(
                stdout,
                "stdout",
                task_id.clone(),
                action_id.clone(),
                security.clone(),
                max_output_bytes,
                sent.clone(),
                truncated_sent.clone(),
                tx.clone(),
            ));
            let stderr_task = tokio::spawn(pump_reader(
                stderr,
                "stderr",
                task_id.clone(),
                action_id.clone(),
                security,
                max_output_bytes,
                sent,
                truncated_sent,
                tx,
            ));

            let started = Instant::now();
            let mut ticker = tokio::time::interval(Duration::from_millis(25));
            let mut child_done = false;
            let mut pipes_done = false;
            let mut exit_code = -1;

            loop {
                if child_done && pipes_done {
                    break;
                }
                tokio::select! {
                    maybe = rx.recv(), if !pipes_done => {
                        match maybe {
                            Some(Ok(chunk)) => yield Ok(chunk),
                            Some(Err(error)) => {
                                let _ = stdout_task.await;
                                let _ = stderr_task.await;
                                yield Err(error);
                                return;
                            }
                            None => {
                                pipes_done = true;
                            }
                        }
                    }
                    _ = ticker.tick(), if !child_done => {
                        if started.elapsed() > spec.timeout {
                            let _ = child.kill().await;
                            let _ = stdout_task.await;
                            let _ = stderr_task.await;
                            yield Err(Status::deadline_exceeded(format!(
                                "runtime: command timed out after {:.1}s",
                                spec.timeout.as_secs_f64()
                            )));
                            return;
                        }
                        match child.try_wait() {
                            Ok(Some(status)) => {
                                exit_code = status.code().unwrap_or(-1);
                                child_done = true;
                            }
                            Ok(None) => {}
                            Err(error) => {
                                let _ = stdout_task.await;
                                let _ = stderr_task.await;
                                yield Err(Status::internal(error.to_string()));
                                return;
                            }
                        }
                    }
                }
            }

            let _ = stdout_task.await;
            let _ = stderr_task.await;
            yield Ok(StreamChunk {
                task_id: task_id.clone(),
                action_id: action_id.clone(),
                data: exit_code.to_string().into_bytes(),
                kind: "exit".into(),
            });
        };

        Ok(Response::new(Box::pin(stream)))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use agentos_sandbox::docker::{DockerConfig, DockerRuntime};
    use agentos_sandbox::native::NativeRuntime;
    use agentos_sandbox::security::{AutonomyLevel, SecurityPolicy};
    use futures::StreamExt;
    use std::fs;
    #[cfg(unix)]
    use std::os::unix::fs::PermissionsExt;
    use std::sync::{Mutex, OnceLock};

    fn test_service() -> RuntimeServiceImpl {
        let runtime = Arc::new(NativeRuntime::new());
        let security = Arc::new(SecurityPolicy {
            autonomy: AutonomyLevel::Supervised,
            ..SecurityPolicy::default()
        });
        let executor = Arc::new(ActionExecutor::new(runtime, security));
        RuntimeServiceImpl::new(executor)
    }

    fn test_service_docker() -> RuntimeServiceImpl {
        let runtime = Arc::new(DockerRuntime::new(DockerConfig::default()));
        let security = Arc::new(SecurityPolicy {
            autonomy: AutonomyLevel::Supervised,
            ..SecurityPolicy::default()
        });
        let executor = Arc::new(ActionExecutor::new(runtime, security));
        RuntimeServiceImpl::new(executor)
    }

    #[cfg(unix)]
    fn path_lock() -> &'static Mutex<()> {
        static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
        LOCK.get_or_init(|| Mutex::new(()))
    }

    #[cfg(unix)]
    struct ScopedPath {
        original: std::ffi::OsString,
        temp_dir: std::path::PathBuf,
    }

    #[cfg(unix)]
    impl ScopedPath {
        fn with_fake_docker(script_body: &str) -> Self {
            let original = std::env::var_os("PATH").unwrap_or_default();
            let temp_dir = std::env::temp_dir().join(format!(
                "agentos-fake-docker-{}",
                std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_nanos()
            ));
            fs::create_dir_all(&temp_dir).expect("create fake docker dir");
            let script_path = temp_dir.join("docker");
            fs::write(&script_path, script_body).expect("write fake docker script");
            let mut perms = fs::metadata(&script_path)
                .expect("stat fake docker script")
                .permissions();
            perms.set_mode(0o755);
            fs::set_permissions(&script_path, perms).expect("chmod fake docker script");
            let new_path = if original.is_empty() {
                temp_dir.as_os_str().to_owned()
            } else {
                let mut joined = std::ffi::OsString::new();
                joined.push(temp_dir.as_os_str());
                joined.push(":");
                joined.push(&original);
                joined
            };
            std::env::set_var("PATH", new_path);
            Self { original, temp_dir }
        }
    }

    #[cfg(unix)]
    impl Drop for ScopedPath {
        fn drop(&mut self) {
            std::env::set_var("PATH", &self.original);
            let _ = fs::remove_dir_all(&self.temp_dir);
        }
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
    async fn grpc_stream_output_emits_chunk_before_process_exit() {
        let svc = test_service();
        let payload = serde_json::json!({"command": "echo first; sleep 0.2; echo second"});
        let req = Request::new(StreamOutputRequest {
            task_id: "t".into(),
            action_id: "action-1".into(),
            payload: serde_json::to_vec(&payload).unwrap(),
        });
        let resp = svc.stream_output(req).await.expect("stream should start");
        let mut stream = resp.into_inner();

        let first = tokio::time::timeout(Duration::from_millis(150), stream.next())
            .await
            .expect("expected first chunk before command completes")
            .expect("expected stream item")
            .expect("expected successful chunk");

        assert_eq!(first.kind, "stdout");
        let text = String::from_utf8_lossy(&first.data);
        assert!(text.contains("first"), "unexpected first chunk: {text}");
    }

    #[tokio::test]
    async fn grpc_stream_output_accepts_legacy_action_id_payload() {
        let svc = test_service();
        let payload = serde_json::json!({"command": "echo legacy"});
        let req = Request::new(StreamOutputRequest {
            task_id: "t".into(),
            action_id: payload.to_string(),
            payload: Vec::new(),
        });
        let resp = svc
            .stream_output(req)
            .await
            .expect("legacy stream should start");
        let mut stream = resp.into_inner();

        let first = stream
            .next()
            .await
            .expect("expected stream item")
            .expect("expected successful chunk");
        assert_eq!(first.kind, "stdout");
        let text = String::from_utf8_lossy(&first.data);
        assert!(text.contains("legacy"), "unexpected legacy chunk: {text}");
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn grpc_stream_output_docker_emits_chunk_before_process_exit() {
        let _guard = path_lock().lock().expect("path lock");
        let _path = ScopedPath::with_fake_docker(
            r#"#!/usr/bin/env python3
import sys, time
sys.stdout.write("docker-first\n")
sys.stdout.flush()
time.sleep(3)
sys.stdout.write("docker-second\n")
sys.stdout.flush()
"#,
        );
        let svc = test_service_docker();
        let payload = serde_json::json!({"command": "echo docker-test"});
        let req = Request::new(StreamOutputRequest {
            task_id: "t".into(),
            action_id: "action-1".into(),
            payload: serde_json::to_vec(&payload).unwrap(),
        });
        let resp = svc.stream_output(req).await.expect("stream should start");
        let mut stream = resp.into_inner();

        let first = tokio::time::timeout(Duration::from_secs(4), stream.next())
            .await
            .expect("expected first docker chunk before process completes")
            .expect("expected stream item")
            .expect("expected successful chunk");

        assert_eq!(first.kind, "stdout");
        let text = String::from_utf8_lossy(&first.data);
        assert!(
            text.contains("docker-first"),
            "unexpected first chunk: {text}"
        );
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
