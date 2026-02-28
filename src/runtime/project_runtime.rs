use anyhow::Result;
use std::collections::HashMap;
use uuid::Uuid;

use crate::core::engine::{explain_merge_route_decision, run_company_flow};
use crate::core::models::{
    GoalContract, MergeReworkRoute, MergeReworkRule, MergeRouteExplanation, ProjectReport,
};
use crate::plugins::PluginRegistry;

pub struct ProjectRuntime {
    plugins: PluginRegistry,
    max_parallel_tasks: usize,
    max_parallel_teams: usize,
    merge_auto_rework: bool,
    max_merge_retries: u32,
    merge_rework_routes: HashMap<String, MergeReworkRoute>,
    merge_rework_rules: Vec<MergeReworkRule>,
    role_failover: bool,
    max_role_attempts: usize,
}

impl ProjectRuntime {
    pub fn new(
        plugins: PluginRegistry,
        max_parallel_tasks: usize,
        max_parallel_teams: usize,
        merge_auto_rework: bool,
        max_merge_retries: u32,
        merge_rework_routes: HashMap<String, MergeReworkRoute>,
        merge_rework_rules: Vec<MergeReworkRule>,
        role_failover: bool,
        max_role_attempts: usize,
    ) -> Self {
        Self {
            plugins,
            max_parallel_tasks,
            max_parallel_teams,
            merge_auto_rework,
            max_merge_retries,
            merge_rework_routes,
            merge_rework_rules,
            role_failover,
            max_role_attempts,
        }
    }

    pub fn team_run(&self, requirement: &str) -> Result<ProjectReport> {
        let goal = GoalContract {
            goal_id: format!("goal_{}", Uuid::new_v4().simple()),
            objective: requirement.to_string(),
            acceptance_criteria: vec![
                "all four gates approved by unanimous board".to_string(),
                "delivery artifacts generated".to_string(),
            ],
        };

        run_company_flow(
            requirement,
            goal,
            self.max_parallel_tasks,
            self.max_parallel_teams,
            self.merge_auto_rework,
            self.max_merge_retries,
            &self.merge_rework_routes,
            &self.merge_rework_rules,
            self.role_failover,
            self.max_role_attempts,
            &self.plugins,
        )
    }

    pub fn explain_routing(&self, requirement: &str, retry_round: u32) -> MergeRouteExplanation {
        explain_merge_route_decision(
            requirement,
            &[],
            retry_round.max(1),
            &self.merge_rework_routes,
            &self.merge_rework_rules,
        )
    }
}

#[cfg(test)]
mod tests {
    use super::ProjectRuntime;
    use crate::runtime::bootstrap::registry_from_profile;
    use crate::runtime::profile::RuntimeProfile;

    #[test]
    fn explain_routing_returns_matched_rule() {
        let profile = RuntimeProfile::default();
        let runtime = ProjectRuntime::new(
            registry_from_profile(&profile).expect("plugins"),
            3,
            1,
            false,
            1,
            profile.merge_rework_routes.clone(),
            profile.merge_rework_rules.clone(),
            false,
            2,
        );

        let explain = runtime.explain_routing("demo [[merge:api-conflict]]", 1);
        assert_eq!(explain.selected_route.route_name, "api-conflict");
        assert!(explain
            .matched_rule
            .as_ref()
            .map(|rule| rule.route_key == "api-conflict")
            .unwrap_or(false));
    }
}
