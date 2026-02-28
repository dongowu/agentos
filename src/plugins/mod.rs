use std::sync::Arc;

use anyhow::Result;

use crate::core::models::{GateId, GateVote, GoalContract, MergeOutcome, TaskNode, TaskReport};

pub mod builtin;

#[derive(Debug, Clone)]
pub struct RoleExecution {
    pub instance_id: String,
    pub summary: String,
    pub artifacts: Vec<String>,
}

#[derive(Debug, Clone)]
pub struct ArbiterDecision {
    pub approved: bool,
    pub note: String,
    pub escalated_to_human: bool,
}

pub trait RoleProvider: Send + Sync {
    fn available_instances(&self, role: &str) -> Vec<String>;
    fn execute(
        &self,
        role: &str,
        instance_id: &str,
        task: &TaskNode,
        goal: &GoalContract,
    ) -> Result<RoleExecution>;
}

pub trait TeamStrategy: Send + Sync {
    fn build_task_graph(&self, requirement: &str, goal: &GoalContract) -> Vec<TaskNode>;
}

pub trait RiskPolicy: Send + Sync {
    fn classify(&self, execution: &RoleExecution) -> String;
}

pub trait GatePolicy: Send + Sync {
    fn evaluate(&self, gate: GateId, reports: &[TaskReport], requirement: &str) -> Vec<GateVote>;
}

pub trait ArbiterPolicy: Send + Sync {
    fn resolve(&self, gate: GateId, votes: &[GateVote]) -> ArbiterDecision;
}

pub trait MergePolicy: Send + Sync {
    fn merge(&self, reports: &[TaskReport], requirement: &str) -> MergeOutcome;
}

#[derive(Clone)]
pub struct PluginRegistry {
    pub role_provider: Arc<dyn RoleProvider>,
    pub team_strategy: Arc<dyn TeamStrategy>,
    pub gate_policy: Arc<dyn GatePolicy>,
    pub arbiter_policy: Arc<dyn ArbiterPolicy>,
    pub risk_policy: Arc<dyn RiskPolicy>,
    pub merge_policy: Arc<dyn MergePolicy>,
}
