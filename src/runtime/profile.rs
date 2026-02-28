use std::collections::HashMap;
use std::fs;
use std::path::Path;

use anyhow::{bail, Context, Result};
use serde::{Deserialize, Serialize};

use crate::core::models::{
    default_merge_rework_routes, default_merge_rework_rules, MergeReworkRoute, MergeReworkRule,
};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RuntimeProfile {
    #[serde(default = "default_gate_policy")]
    pub gate_policy: String,
    #[serde(default = "default_arbiter_policy")]
    pub arbiter_policy: String,
    #[serde(default = "default_merge_policy")]
    pub merge_policy: String,
    #[serde(default)]
    pub merge_auto_rework: bool,
    #[serde(default = "default_max_merge_retries")]
    pub max_merge_retries: u32,
    #[serde(default = "default_routes")]
    pub merge_rework_routes: HashMap<String, MergeReworkRoute>,
    #[serde(default = "default_rules")]
    pub merge_rework_rules: Vec<MergeReworkRule>,
    #[serde(default)]
    pub role_failover: bool,
    #[serde(default = "default_max_role_attempts")]
    pub max_role_attempts: usize,
    #[serde(default)]
    pub role_instances: HashMap<String, Vec<String>>,
    #[serde(default = "default_team_topology")]
    pub team_topology: String,
    #[serde(default = "default_max_parallel_teams")]
    pub max_parallel_teams: usize,
}

fn default_gate_policy() -> String {
    "unanimous".to_string()
}

fn default_arbiter_policy() -> String {
    "two_round".to_string()
}

fn default_merge_policy() -> String {
    "strict".to_string()
}

fn default_max_merge_retries() -> u32 {
    1
}

fn default_routes() -> HashMap<String, MergeReworkRoute> {
    default_merge_rework_routes()
}

fn default_rules() -> Vec<MergeReworkRule> {
    default_merge_rework_rules()
}

fn default_max_role_attempts() -> usize {
    2
}

fn default_team_topology() -> String {
    "single".to_string()
}

fn default_max_parallel_teams() -> usize {
    1
}

impl Default for RuntimeProfile {
    fn default() -> Self {
        Self {
            gate_policy: default_gate_policy(),
            arbiter_policy: default_arbiter_policy(),
            merge_policy: default_merge_policy(),
            merge_auto_rework: false,
            max_merge_retries: default_max_merge_retries(),
            merge_rework_routes: default_routes(),
            merge_rework_rules: default_rules(),
            role_failover: false,
            max_role_attempts: default_max_role_attempts(),
            role_instances: HashMap::new(),
            team_topology: default_team_topology(),
            max_parallel_teams: default_max_parallel_teams(),
        }
    }
}

impl RuntimeProfile {
    fn validate(&self) -> Result<()> {
        if !self.merge_rework_routes.contains_key("generic") {
            bail!("merge_rework_routes must define a 'generic' route");
        }

        for rule in &self.merge_rework_rules {
            if !rule.condition_mode.eq_ignore_ascii_case("all")
                && !rule.condition_mode.eq_ignore_ascii_case("any")
            {
                bail!(
                    "invalid condition_mode '{}' for route_key '{}' (expected 'all' or 'any')",
                    rule.condition_mode,
                    rule.route_key
                );
            }

            if !self.merge_rework_routes.contains_key(&rule.route_key) {
                bail!(
                    "merge_rework_rules references unknown route_key '{}'",
                    rule.route_key
                );
            }
        }

        Ok(())
    }

    pub fn load(path: Option<&Path>) -> Result<Self> {
        if let Some(path) = path {
            let raw = fs::read_to_string(path)
                .with_context(|| format!("failed to read runtime profile {}", path.display()))?;
            let parsed = serde_yaml::from_str::<RuntimeProfile>(&raw)
                .with_context(|| format!("failed to parse runtime profile {}", path.display()))?;
            parsed.validate()?;
            return Ok(parsed);
        }
        let profile = Self::default();
        profile.validate()?;
        Ok(profile)
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

    pub fn with_merge_policy(mut self, policy: Option<String>) -> Self {
        if let Some(policy) = policy {
            self.merge_policy = policy;
        }
        self
    }

    pub fn with_merge_auto_rework(mut self, enabled: bool) -> Self {
        if enabled {
            self.merge_auto_rework = true;
        }
        self
    }

    pub fn with_max_merge_retries(mut self, max_retries: Option<u32>) -> Self {
        if let Some(max_retries) = max_retries {
            self.max_merge_retries = max_retries.max(1);
        }
        self
    }

    pub fn with_role_failover(mut self, enabled: bool) -> Self {
        if enabled {
            self.role_failover = true;
        }
        self
    }

    pub fn with_max_role_attempts(mut self, attempts: Option<usize>) -> Self {
        if let Some(attempts) = attempts {
            self.max_role_attempts = attempts.max(1);
        }
        self
    }

    pub fn with_team_topology(mut self, topology: Option<String>) -> Self {
        if let Some(topology) = topology {
            self.team_topology = topology;
        }
        self
    }

    pub fn with_max_parallel_teams(mut self, max_parallel_teams: Option<usize>) -> Self {
        if let Some(max_parallel_teams) = max_parallel_teams {
            self.max_parallel_teams = max_parallel_teams.max(1);
        }
        self
    }
}

#[cfg(test)]
mod tests {
    use super::RuntimeProfile;

    #[test]
    fn rejects_unknown_condition_mode() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_mode = "xor".to_string();
        let err = profile.validate().expect_err("should fail");
        assert!(err.to_string().contains("invalid condition_mode"));
    }

    #[test]
    fn rejects_rule_with_unknown_route_key() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].route_key = "missing-route".to_string();
        let err = profile.validate().expect_err("should fail");
        assert!(err.to_string().contains("unknown route_key"));
    }

    #[test]
    fn accepts_uppercase_condition_mode() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_mode = "ANY".to_string();
        profile.validate().expect("should pass");
    }
}
