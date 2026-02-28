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
    #[serde(default = "default_llm_adapter")]
    pub llm_adapter: String,
    #[serde(default = "default_llm_model")]
    pub llm_model: String,
    #[serde(default)]
    pub llm_script_command: Option<String>,
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

fn default_llm_adapter() -> String {
    "mock".to_string()
}

fn default_llm_model() -> String {
    "orchestrator-sim".to_string()
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
            llm_adapter: default_llm_adapter(),
            llm_model: default_llm_model(),
            llm_script_command: None,
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

        if self.llm_adapter != "mock" && self.llm_adapter != "script" {
            bail!(
                "invalid llm_adapter '{}' (expected 'mock' or 'script')",
                self.llm_adapter
            );
        }

        if self.llm_adapter == "script" {
            let command = self
                .llm_script_command
                .as_ref()
                .map(|value| value.trim())
                .unwrap_or("");
            if command.is_empty() {
                bail!("llm_script_command is required when llm_adapter='script'");
            }
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

            if let Some(required) = rule.required_risk_level.as_ref() {
                validate_risk_level(required).with_context(|| {
                    format!(
                        "invalid required_risk_level for route_key '{}': {}",
                        rule.route_key, required
                    )
                })?;
            }

            if let Some(expression) = rule.condition_expression.as_ref() {
                validate_condition_expression(expression).with_context(|| {
                    format!(
                        "invalid condition_expression for route_key '{}': {}",
                        rule.route_key, expression
                    )
                })?;
            }
        }

        let mut priorities: HashMap<u32, usize> = HashMap::new();
        for rule in &self.merge_rework_rules {
            *priorities.entry(rule.priority).or_insert(0) += 1;
        }
        if let Some((priority, _count)) = priorities.into_iter().find(|(_, count)| *count > 1) {
            bail!(
                "merge_rework_rules contains duplicate priority '{}' (priorities must be unique)",
                priority
            );
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

    pub fn with_llm_adapter(mut self, adapter: Option<String>) -> Self {
        if let Some(adapter) = adapter {
            self.llm_adapter = adapter;
        }
        self
    }

    pub fn with_llm_model(mut self, model: Option<String>) -> Self {
        if let Some(model) = model {
            self.llm_model = model;
        }
        self
    }

    pub fn with_llm_script_command(mut self, command: Option<String>) -> Self {
        if let Some(command) = command {
            self.llm_script_command = Some(command);
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

fn validate_condition_expression(expression: &str) -> Result<()> {
    let expr = expression.trim();
    if expr.is_empty() {
        bail!("expression must contain at least one branch");
    }
    if !has_balanced_parentheses(expr) {
        bail!("expression has unbalanced parentheses");
    }

    validate_condition_expression_recursive(expr)
}

fn validate_condition_expression_recursive(expression: &str) -> Result<()> {
    let expr = strip_outer_parentheses(expression.trim());
    if expr.is_empty() {
        bail!("expression must contain at least one branch");
    }

    let or_parts = split_top_level(expr, "||");
    if or_parts.len() > 1 {
        for part in or_parts {
            validate_condition_expression_recursive(part)?;
        }
        return Ok(());
    }

    let and_parts = split_top_level(expr, "&&");
    if and_parts.len() > 1 {
        for part in and_parts {
            validate_condition_expression_recursive(part)?;
        }
        return Ok(());
    }

    validate_condition_atom(expr)
}

fn split_top_level<'a>(expression: &'a str, operator: &str) -> Vec<&'a str> {
    let mut parts = Vec::new();
    let mut depth = 0usize;
    let mut start = 0usize;
    let bytes = expression.as_bytes();
    let op_bytes = operator.as_bytes();
    let mut i = 0usize;

    while i < bytes.len() {
        match bytes[i] as char {
            '(' => depth += 1,
            ')' => depth = depth.saturating_sub(1),
            _ => {}
        }

        if depth == 0
            && i + op_bytes.len() <= bytes.len()
            && &bytes[i..i + op_bytes.len()] == op_bytes
        {
            let part = expression[start..i].trim();
            if !part.is_empty() {
                parts.push(part);
            }
            i += op_bytes.len();
            start = i;
            continue;
        }

        i += 1;
    }

    let tail = expression[start..].trim();
    if !tail.is_empty() {
        parts.push(tail);
    }

    if parts.is_empty() {
        vec![expression]
    } else {
        parts
    }
}

fn strip_outer_parentheses(mut expression: &str) -> &str {
    loop {
        let trimmed = expression.trim();
        if !(trimmed.starts_with('(') && trimmed.ends_with(')')) {
            return trimmed;
        }

        let mut depth = 0usize;
        let mut encloses_all = true;
        for (idx, ch) in trimmed.char_indices() {
            match ch {
                '(' => depth += 1,
                ')' => {
                    depth = depth.saturating_sub(1);
                    if depth == 0 && idx != trimmed.len() - 1 {
                        encloses_all = false;
                        break;
                    }
                }
                _ => {}
            }
        }

        if !encloses_all {
            return trimmed;
        }
        expression = &trimmed[1..trimmed.len() - 1];
    }
}

fn has_balanced_parentheses(expression: &str) -> bool {
    let mut depth = 0usize;
    for ch in expression.chars() {
        match ch {
            '(' => depth += 1,
            ')' => {
                if depth == 0 {
                    return false;
                }
                depth -= 1;
            }
            _ => {}
        }
    }
    depth == 0
}

fn validate_condition_atom(atom: &str) -> Result<()> {
    let atom = atom.trim();

    if atom.eq_ignore_ascii_case("true") || atom.eq_ignore_ascii_case("false") {
        return Ok(());
    }

    if let Some(inner) = atom.strip_prefix('!') {
        if inner.trim().is_empty() {
            bail!("unsupported condition atom '{}'", atom);
        }
        return validate_condition_atom(inner.trim());
    }

    if let Some(value) = atom.strip_prefix("risk==") {
        return validate_risk_level(value.trim());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "risk", "==") {
        return validate_risk_level(rhs);
    }

    if let Some(value) = atom.strip_prefix("risk>=") {
        return validate_risk_level(value.trim());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "risk", ">=") {
        return validate_risk_level(rhs);
    }

    if let Some(value) = atom.strip_prefix("risk<=") {
        return validate_risk_level(value.trim());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "risk", "<=") {
        return validate_risk_level(rhs);
    }

    if let Some(value) = atom.strip_prefix("retry>=") {
        value
            .trim()
            .parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "retry", ">=") {
        rhs.parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", rhs))?;
        return Ok(());
    }

    if let Some(value) = atom.strip_prefix("retry>") {
        value
            .trim()
            .parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "retry", ">") {
        rhs.parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", rhs))?;
        return Ok(());
    }

    if let Some(value) = atom.strip_prefix("retry<=") {
        value
            .trim()
            .parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "retry", "<=") {
        rhs.parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", rhs))?;
        return Ok(());
    }

    if let Some(value) = atom.strip_prefix("retry<") {
        value
            .trim()
            .parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "retry", "<") {
        rhs.parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", rhs))?;
        return Ok(());
    }

    if let Some(value) = atom.strip_prefix("retry==") {
        value
            .trim()
            .parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "retry", "==") {
        rhs.parse::<u32>()
            .with_context(|| format!("invalid retry bound '{}'", rhs))?;
        return Ok(());
    }

    if let Some(value) = atom.strip_prefix("team_load<=") {
        value
            .trim()
            .parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "team_load", "<=") {
        rhs.parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", rhs))?;
        return Ok(());
    }

    if let Some(value) = atom.strip_prefix("team_load<") {
        value
            .trim()
            .parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "team_load", "<") {
        rhs.parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", rhs))?;
        return Ok(());
    }

    if let Some(value) = atom.strip_prefix("team_load>=") {
        value
            .trim()
            .parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "team_load", ">=") {
        rhs.parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", rhs))?;
        return Ok(());
    }

    if let Some(value) = atom.strip_prefix("team_load>") {
        value
            .trim()
            .parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "team_load", ">") {
        rhs.parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", rhs))?;
        return Ok(());
    }

    if let Some(value) = atom.strip_prefix("team_load==") {
        value
            .trim()
            .parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", value.trim()))?;
        return Ok(());
    }
    if let Some((_lhs, rhs)) = split_infix_atom(atom, "team_load", "==") {
        rhs.parse::<usize>()
            .with_context(|| format!("invalid team_load bound '{}'", rhs))?;
        return Ok(());
    }

    bail!("unsupported condition atom '{}'", atom)
}

fn validate_risk_level(level: &str) -> Result<()> {
    let normalized = level.trim().to_ascii_lowercase();
    if normalized == "low" || normalized == "medium" || normalized == "high" {
        return Ok(());
    }
    bail!("unsupported risk level '{}'", level)
}

fn split_infix_atom<'a>(atom: &'a str, field: &str, op: &str) -> Option<(&'a str, &'a str)> {
    let parts: Vec<&str> = atom.splitn(2, op).collect();
    if parts.len() != 2 {
        return None;
    }
    let lhs = parts[0].trim();
    let rhs = parts[1].trim();
    if lhs.eq_ignore_ascii_case(field) && !rhs.is_empty() {
        Some((lhs, rhs))
    } else {
        None
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

    #[test]
    fn rejects_invalid_condition_expression_atom() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression = Some("foo==bar".to_string());
        let err = profile.validate().expect_err("should fail");
        assert!(err.to_string().contains("invalid condition_expression"));
    }

    #[test]
    fn accepts_valid_condition_expression() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression =
            Some("risk>=medium && retry>=1 || team_load<=3".to_string());
        profile.validate().expect("should pass");
    }

    #[test]
    fn accepts_retry_equal_and_team_load_equal_expression() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression =
            Some("retry==1 && team_load==2".to_string());
        profile.validate().expect("should pass");
    }

    #[test]
    fn rejects_duplicate_rule_priorities() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[1].priority = profile.merge_rework_rules[0].priority;
        let err = profile.validate().expect_err("should fail");
        assert!(err.to_string().contains("duplicate priority"));
    }

    #[test]
    fn accepts_risk_equal_expression() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression = Some("risk==low".to_string());
        profile.validate().expect("should pass");
    }

    #[test]
    fn accepts_case_insensitive_required_risk_level() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].required_risk_level = Some("HIGH".to_string());
        profile.validate().expect("should pass");
    }

    #[test]
    fn accepts_negated_expression_atom() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression = Some("!risk==high".to_string());
        profile.validate().expect("should pass");
    }

    #[test]
    fn accepts_boolean_literal_expression() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression = Some("true && !false".to_string());
        profile.validate().expect("should pass");
    }

    #[test]
    fn accepts_parenthesized_condition_expression() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression =
            Some("(risk==low || retry==2) && !false".to_string());
        profile.validate().expect("should pass");
    }

    #[test]
    fn accepts_strict_inequality_expression() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression =
            Some("retry>0 && retry<3 && team_load>1 && team_load<5".to_string());
        profile.validate().expect("should pass");
    }

    #[test]
    fn accepts_whitespace_infix_expression() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression =
            Some("retry >= 1 && team_load <= 3 && risk == low".to_string());
        profile.validate().expect("should pass");
    }

    #[test]
    fn rejects_unbalanced_parentheses_expression() {
        let mut profile = RuntimeProfile::default();
        profile.merge_rework_rules[0].condition_expression = Some("(risk==low".to_string());
        let err = profile.validate().expect_err("should fail");
        assert!(err.to_string().contains("invalid condition_expression"));
    }

    #[test]
    fn rejects_unknown_llm_adapter() {
        let mut profile = RuntimeProfile::default();
        profile.llm_adapter = "other".to_string();
        let err = profile.validate().expect_err("should fail");
        assert!(err.to_string().contains("invalid llm_adapter"));
    }

    #[test]
    fn requires_script_command_for_script_adapter() {
        let mut profile = RuntimeProfile::default();
        profile.llm_adapter = "script".to_string();
        let err = profile.validate().expect_err("should fail");
        assert!(err.to_string().contains("llm_script_command is required"));
    }
}
