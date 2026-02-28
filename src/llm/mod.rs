use std::process::Command;

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
}

impl ScriptLlmAdapter {
    pub fn new(command: String) -> Self {
        Self { command }
    }
}

impl LlmAdapter for ScriptLlmAdapter {
    fn generate_summary(&self, request: &LlmRequest) -> Result<String> {
        let output = Command::new("bash")
            .arg("-lc")
            .arg(&self.command)
            .env("ORCH_ROLE", &request.role)
            .env("ORCH_INSTANCE", &request.instance_id)
            .env("ORCH_TASK_ID", &request.task_id)
            .env("ORCH_TASK_TITLE", &request.task_title)
            .env("ORCH_GOAL_ID", &request.goal_id)
            .env("ORCH_OBJECTIVE", &request.objective)
            .env("ORCH_MODEL", &request.model)
            .output()
            .with_context(|| "failed to execute llm script command")?;

        if !output.status.success() {
            return Err(anyhow!(
                "llm script command failed with status {}",
                output.status
            ));
        }

        let summary = String::from_utf8(output.stdout)
            .with_context(|| "llm script output is not valid utf-8")?
            .trim()
            .to_string();
        if summary.is_empty() {
            return Err(anyhow!("llm script returned empty summary"));
        }
        Ok(summary)
    }
}

#[cfg(test)]
mod tests {
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
        let adapter =
            ScriptLlmAdapter::new("printf '%s:%s' \"$ORCH_ROLE\" \"$ORCH_TASK_ID\"".to_string());
        let result = adapter.generate_summary(&request()).expect("summary");
        assert_eq!(result, "coder:implementation");
    }
}
