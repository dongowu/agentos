use std::fs;
use std::path::Path;

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RuntimeProfile {
    #[serde(default = "default_gate_policy")]
    pub gate_policy: String,
    #[serde(default = "default_arbiter_policy")]
    pub arbiter_policy: String,
}

fn default_gate_policy() -> String {
    "unanimous".to_string()
}

fn default_arbiter_policy() -> String {
    "two_round".to_string()
}

impl Default for RuntimeProfile {
    fn default() -> Self {
        Self {
            gate_policy: default_gate_policy(),
            arbiter_policy: default_arbiter_policy(),
        }
    }
}

impl RuntimeProfile {
    pub fn load(path: Option<&Path>) -> Result<Self> {
        if let Some(path) = path {
            let raw = fs::read_to_string(path)
                .with_context(|| format!("failed to read runtime profile {}", path.display()))?;
            let parsed = serde_yaml::from_str::<RuntimeProfile>(&raw)
                .with_context(|| format!("failed to parse runtime profile {}", path.display()))?;
            return Ok(parsed);
        }
        Ok(Self::default())
    }

    pub fn with_gate_policy(mut self, policy: Option<String>) -> Self {
        if let Some(policy) = policy {
            self.gate_policy = policy;
        }
        self
    }

    pub fn with_arbiter_policy(mut self, policy: Option<String>) -> Self {
        if let Some(policy) = policy {
            self.arbiter_policy = policy;
        }
        self
    }
}
