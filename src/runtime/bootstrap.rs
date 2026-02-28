use anyhow::{bail, Result};
use std::sync::Arc;

use crate::plugins::builtin::{
    BuiltinRiskPolicy, BuiltinRoleProvider, BuiltinTeamStrategy, EscalateImmediatelyArbiter,
    MajorityGatePolicy, TwoRoundArbiter, UnanimousGatePolicy,
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

    Ok(PluginRegistry {
        role_provider: Arc::new(BuiltinRoleProvider::from_overrides(
            profile.role_instances.clone(),
        )),
        team_strategy: Arc::new(BuiltinTeamStrategy::new(profile.team_topology.clone())),
        gate_policy,
        arbiter_policy,
        risk_policy: Arc::new(BuiltinRiskPolicy),
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
}
