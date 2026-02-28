use anyhow::{bail, Result};
use std::sync::Arc;

use crate::llm::{LlmAdapter, MockLlmAdapter, ScriptLlmAdapter};
use crate::plugins::builtin::{
    BuiltinRiskPolicy, BuiltinRoleProvider, BuiltinTeamStrategy, EscalateImmediatelyArbiter,
    FastMergePolicy, MajorityGatePolicy, StrictMergePolicy, TwoRoundArbiter, UnanimousGatePolicy,
};
use crate::plugins::PluginRegistry;
use crate::runtime::profile::RuntimeProfile;

pub fn registry_from_profile(profile: &RuntimeProfile) -> Result<PluginRegistry> {
    let gate_policy = match profile.gate_policy.as_str() {
        "unanimous" => Arc::new(UnanimousGatePolicy) as _,
        "majority" => Arc::new(MajorityGatePolicy) as _,
        other => bail!("unsupported gate policy: {}", other),
    };

    let arbiter_policy = match profile.arbiter_policy.as_str() {
        "two_round" => Arc::new(TwoRoundArbiter) as _,
        "immediate_escalation" => Arc::new(EscalateImmediatelyArbiter) as _,
        other => bail!("unsupported arbiter policy: {}", other),
    };

    let merge_policy = match profile.merge_policy.as_str() {
        "strict" => Arc::new(StrictMergePolicy) as _,
        "fast" => Arc::new(FastMergePolicy) as _,
        other => bail!("unsupported merge policy: {}", other),
    };

    let llm_adapter: Arc<dyn LlmAdapter> = match profile.llm_adapter.as_str() {
        "mock" => Arc::new(MockLlmAdapter),
        "script" => {
            let command = profile
                .llm_script_command
                .as_ref()
                .map(|value| value.trim())
                .unwrap_or("");
            if command.is_empty() {
                bail!("llm_script_command is required when llm_adapter='script'");
            }
            Arc::new(ScriptLlmAdapter::new(command.to_string()))
        }
        other => bail!("unsupported llm adapter: {}", other),
    };

    Ok(PluginRegistry {
        role_provider: Arc::new(BuiltinRoleProvider::from_overrides(
            profile.role_instances.clone(),
            llm_adapter,
            profile.llm_model.clone(),
        )),
        team_strategy: Arc::new(BuiltinTeamStrategy::new(profile.team_topology.clone())),
        gate_policy,
        arbiter_policy,
        risk_policy: Arc::new(BuiltinRiskPolicy),
        merge_policy,
    })
}

#[cfg(test)]
mod tests {
    use super::registry_from_profile;
    use crate::runtime::profile::RuntimeProfile;

    #[test]
    fn rejects_unknown_policy_names() {
        let profile = RuntimeProfile {
            gate_policy: "does_not_exist".to_string(),
            arbiter_policy: "two_round".to_string(),
            merge_policy: "strict".to_string(),
            llm_adapter: "mock".to_string(),
            llm_model: "orchestrator-sim".to_string(),
            llm_script_command: None,
            merge_auto_rework: false,
            max_merge_retries: 1,
            merge_rework_routes: crate::core::models::default_merge_rework_routes(),
            merge_rework_rules: crate::core::models::default_merge_rework_rules(),
            role_failover: false,
            max_role_attempts: 2,
            role_instances: Default::default(),
            team_topology: "single".to_string(),
            max_parallel_teams: 1,
        };

        let err = match registry_from_profile(&profile) {
            Ok(_) => panic!("must fail"),
            Err(err) => err,
        };
        assert!(err.to_string().contains("unsupported gate policy"));
    }

    #[test]
    fn rejects_script_adapter_without_command() {
        let mut profile = RuntimeProfile::default();
        profile.llm_adapter = "script".to_string();
        profile.llm_script_command = None;

        let err = match registry_from_profile(&profile) {
            Ok(_) => panic!("must fail"),
            Err(err) => err,
        };
        assert!(err.to_string().contains("llm_script_command is required"));
    }
}
