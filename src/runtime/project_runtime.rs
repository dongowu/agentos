use anyhow::Result;
use std::collections::HashMap;
use uuid::Uuid;

use crate::core::engine::run_company_flow;
use crate::core::models::{GoalContract, MergeReworkRoute, ProjectReport};
use crate::plugins::PluginRegistry;

pub struct ProjectRuntime {
    plugins: PluginRegistry,
    max_parallel_tasks: usize,
    max_parallel_teams: usize,
    merge_auto_rework: bool,
    max_merge_retries: u32,
    merge_rework_routes: HashMap<String, MergeReworkRoute>,
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
            self.role_failover,
            self.max_role_attempts,
            &self.plugins,
        )
    }
}
