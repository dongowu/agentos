use std::sync::Arc;

use crate::plugins::builtin::{
    BuiltinRiskPolicy, BuiltinRoleProvider, BuiltinTeamStrategy, TwoRoundArbiter,
    UnanimousGatePolicy,
};
use crate::plugins::PluginRegistry;

pub fn default_registry() -> PluginRegistry {
    PluginRegistry {
        role_provider: Arc::new(BuiltinRoleProvider),
        team_strategy: Arc::new(BuiltinTeamStrategy),
        gate_policy: Arc::new(UnanimousGatePolicy),
        arbiter_policy: Arc::new(TwoRoundArbiter),
        risk_policy: Arc::new(BuiltinRiskPolicy),
    }
}
