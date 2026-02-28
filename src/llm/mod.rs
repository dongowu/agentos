use std::io::Read;
use std::process::{Command, Stdio};
use std::time::Duration;

use anyhow::{anyhow, Context, Result};

#[derive(Debug, Clone)]
pub struct LlmRequest {
    pub role: String,
    pub instance_id: String,
    pub task_id: String,
    pub task_title: String,
    pub goal_id: String,
    pub objective: String,
    pub model: String,
}

pub trait LlmAdapter: Send + Sync {
    fn generate_summary(&self, request: &LlmRequest) -> Result<String>;
}

pub struct MockLlmAdapter;

impl LlmAdapter for MockLlmAdapter {
    fn generate_summary(&self, request: &LlmRequest) -> Result<String> {
        Ok(format!(
            "[{}] {} ({}) completed '{}' for goal {}",
            request.model, request.role, request.instance_id, request.task_title, request.goal_id
        ))
    }
}

pub struct ScriptLlmAdapter {
    command: String,
    max_attempts: u32,
    timeout_ms: Option<u64>,
}

impl ScriptLlmAdapter {
    pub fn new(command: String, max_attempts: u32, timeout_ms: Option<u64>) -> Self {
        Self {
            command,
            max_attempts: max_attempts.max(1),
            timeout_ms,
        }
    }
}

impl LlmAdapter for ScriptLlmAdapter {
    fn generate_summary(&self, request: &LlmRequest) -> Result<String> {
        let mut last_error = None;

        for attempt in 1..=self.max_attempts {
            let output = match self.execute_with_timeout(request) {
                Ok(output) => output,
                Err(err) => {
                    last_error = Some(anyhow!(
                        "llm script command execution failed on attempt {}/{}: {}",
                        attempt,
                        self.max_attempts,
                        err
                    ));
                    continue;
                }
            };

            if !output.status.success() {
                let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
                last_error = Some(anyhow!(
                    "llm script command failed on attempt {}/{} with status {}{}",
                    attempt,
                    self.max_attempts,
                    output.status,
                    if stderr.is_empty() {
                        "".to_string()
                    } else {
                        format!(": {}", stderr)
                    }
                ));
                continue;
            }

            let summary = String::from_utf8(output.stdout)
                .with_context(|| "llm script output is not valid utf-8")?
                .trim()
                .to_string();
            if summary.is_empty() {
                last_error = Some(anyhow!(
                    "llm script returned empty summary on attempt {}/{}",
                    attempt,
                    self.max_attempts
                ));
                continue;
            }

            return Ok(summary);
        }

        Err(last_error.unwrap_or_else(|| anyhow!("llm script command failed")))
    }
}

impl ScriptLlmAdapter {
    fn execute_with_timeout(&self, request: &LlmRequest) -> Result<std::process::Output> {
        let mut cmd = Command::new("bash");
        cmd.arg("-lc")
            .arg(&self.command)
            .env("ORCH_ROLE", &request.role)
            .env("ORCH_INSTANCE", &request.instance_id)
            .env("ORCH_TASK_ID", &request.task_id)
            .env("ORCH_TASK_TITLE", &request.task_title)
            .env("ORCH_GOAL_ID", &request.goal_id)
            .env("ORCH_OBJECTIVE", &request.objective)
            .env("ORCH_MODEL", &request.model)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());

        if let Some(timeout_ms) = self.timeout_ms {
            let mut child = cmd
                .spawn()
                .with_context(|| "failed to spawn llm script command")?;
            let timeout = Duration::from_millis(timeout_ms);

            match child.wait_timeout(timeout) {
                Ok(Some(status)) => {
                    let stdout = child
                        .stdout
                        .take()
                        .map(|mut s| {
                            let mut buf = String::new();
                            let _ = s.read_to_string(&mut buf);
                            buf
                        })
                        .unwrap_or_default();
                    let stderr = child
                        .stderr
                        .take()
                        .map(|mut s| {
                            let mut buf = String::new();
                            let _ = s.read_to_string(&mut buf);
                            buf
                        })
                        .unwrap_or_default();

                    return Ok(std::process::Output {
                        status,
                        stdout: stdout.into_bytes(),
                        stderr: stderr.into_bytes(),
                    });
                }
                Ok(None) => {
                    let _ = child.kill();
                    return Err(anyhow!(
                        "llm script command timed out after {}ms",
                        timeout_ms
                    ));
                }
                Err(e) => {
                    return Err(anyhow!("failed to wait on llm script: {}", e));
                }
            }
        } else {
            cmd.output()
                .with_context(|| "failed to execute llm script command")
        }
    }
}

trait ChildExt {
    fn wait_timeout(
        &mut self,
        timeout: Duration,
    ) -> std::io::Result<Option<std::process::ExitStatus>>;
}

impl ChildExt for std::process::Child {
    fn wait_timeout(
        &mut self,
        timeout: Duration,
    ) -> std::io::Result<Option<std::process::ExitStatus>> {
        let start = std::time::Instant::now();
        let poll_interval = Duration::from_millis(100);

        loop {
            match self.try_wait()? {
                Some(status) => return Ok(Some(status)),
                None => {
                    if start.elapsed() >= timeout {
                        return Ok(None);
                    }
                    std::thread::sleep(poll_interval);
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use std::fs;

    use super::{LlmAdapter, LlmRequest, MockLlmAdapter, ScriptLlmAdapter};

    fn request() -> LlmRequest {
        LlmRequest {
            role: "coder".to_string(),
            instance_id: "coder.primary".to_string(),
            task_id: "implementation".to_string(),
            task_title: "Implement login flow".to_string(),
            goal_id: "goal_123".to_string(),
            objective: "ship login".to_string(),
            model: "mock-model".to_string(),
        }
    }

    #[test]
    fn mock_adapter_returns_summary() {
        let adapter = MockLlmAdapter;
        let result = adapter.generate_summary(&request()).expect("summary");
        assert!(result.contains("coder"));
        assert!(result.contains("Implement login flow"));
    }

    #[test]
    fn script_adapter_reads_environment() {
        let adapter = ScriptLlmAdapter::new(
            "printf '%s:%s' \"$ORCH_ROLE\" \"$ORCH_TASK_ID\"".to_string(),
            1,
            None,
        );
        let result = adapter.generate_summary(&request()).expect("summary");
        assert_eq!(result, "coder:implementation");
    }

    #[test]
    fn script_adapter_retries_until_success() {
        let marker =
            std::env::temp_dir().join(format!("orch-llm-retry-{}.txt", uuid::Uuid::new_v4()));
        let marker_path = marker
            .to_str()
            .expect("temp path must be utf-8")
            .replace('"', "\\\"");
        let command = format!(
            "count=$(cat \"{marker_path}\" 2>/dev/null || printf '0'); next=$((count+1)); printf '%s' \"$next\" > \"{marker_path}\"; if [ \"$next\" -lt 2 ]; then exit 1; fi; printf 'ok'"
        );
        let adapter = ScriptLlmAdapter::new(command, 2, None);

        let result = adapter.generate_summary(&request()).expect("summary");
        assert_eq!(result, "ok");

        let _ = fs::remove_file(marker);
    }

    #[test]
    fn script_adapter_returns_last_error_after_retries() {
        let adapter = ScriptLlmAdapter::new("echo nope 1>&2; exit 9".to_string(), 2, None);
        let err = adapter
            .generate_summary(&request())
            .expect_err("should fail");
        assert!(err.to_string().contains("attempt 2/2"));
    }

    #[test]
    fn script_adapter_respects_timeout() {
        let adapter = ScriptLlmAdapter::new("sleep 2; printf ok".to_string(), 1, Some(100));
        let err = adapter
            .generate_summary(&request())
            .expect_err("should fail");
        assert!(err.to_string().contains("timed out"));
    }

    #[test]
    fn script_adapter_succeeds_within_timeout() {
        let adapter = ScriptLlmAdapter::new("printf ok".to_string(), 1, Some(5000));
        let result = adapter.generate_summary(&request()).expect("summary");
        assert_eq!(result, "ok");
    }
}
