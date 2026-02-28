use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum Department {
    Product,
    Engineering,
    Qa,
    Security,
    Ops,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GoalContract {
    pub goal_id: String,
    pub objective: String,
    pub acceptance_criteria: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskNode {
    pub id: String,
    pub team_id: String,
    pub title: String,
    pub owner_role: String,
    pub department: Department,
    pub depends_on: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskReport {
    pub task_id: String,
    pub team_id: String,
    pub role: String,
    pub summary: String,
    pub risk_level: String,
    pub artifacts: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum GateId {
    Intake,
    Freeze,
    Release,
    Closure,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GateVote {
    pub department: Department,
    pub approved: bool,
    pub reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GateOutcome {
    pub gate: GateId,
    pub approved: bool,
    pub votes: Vec<GateVote>,
    pub arbitration_note: Option<String>,
    pub escalated_to_human: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MergeOutcome {
    pub approved: bool,
    pub attempts: u32,
    pub note: String,
    pub escalated_to_human: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MergeReworkRoute {
    pub route_name: String,
    pub task_suffix: String,
    pub team_id: String,
    pub role: String,
    pub actor_summary: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MergeReworkRule {
    pub marker: String,
    pub route_key: String,
    pub priority: u32,
    #[serde(default = "default_condition_mode")]
    pub condition_mode: String,
    #[serde(default)]
    pub required_risk_level: Option<String>,
    #[serde(default)]
    pub min_retry_round: Option<u32>,
    #[serde(default)]
    pub max_team_load: Option<usize>,
}

fn default_condition_mode() -> String {
    "all".to_string()
}

pub fn default_merge_rework_routes() -> HashMap<String, MergeReworkRoute> {
    HashMap::from([
        (
            "generic".to_string(),
            MergeReworkRoute {
                route_name: "generic".to_string(),
                task_suffix: "generic".to_string(),
                team_id: "program_board".to_string(),
                role: "supervisor@supervisor.primary".to_string(),
                actor_summary: "supervisor".to_string(),
            },
        ),
        (
            "code-conflict".to_string(),
            MergeReworkRoute {
                route_name: "code-conflict".to_string(),
                task_suffix: "code".to_string(),
                team_id: "platform_team".to_string(),
                role: "architect@architect.primary".to_string(),
                actor_summary: "platform architect".to_string(),
            },
        ),
        (
            "api-conflict".to_string(),
            MergeReworkRoute {
                route_name: "api-conflict".to_string(),
                task_suffix: "api".to_string(),
                team_id: "feature_team".to_string(),
                role: "architect@architect.primary".to_string(),
                actor_summary: "feature architect".to_string(),
            },
        ),
        (
            "test-conflict".to_string(),
            MergeReworkRoute {
                route_name: "test-conflict".to_string(),
                task_suffix: "test".to_string(),
                team_id: "qa_team".to_string(),
                role: "tester@tester.primary".to_string(),
                actor_summary: "qa lead".to_string(),
            },
        ),
    ])
}

pub fn default_merge_rework_rules() -> Vec<MergeReworkRule> {
    vec![
        MergeReworkRule {
            marker: "[[merge:code-conflict]]".to_string(),
            route_key: "code-conflict".to_string(),
            priority: 10,
            condition_mode: default_condition_mode(),
            required_risk_level: None,
            min_retry_round: None,
            max_team_load: None,
        },
        MergeReworkRule {
            marker: "[[merge:api-conflict]]".to_string(),
            route_key: "api-conflict".to_string(),
            priority: 20,
            condition_mode: default_condition_mode(),
            required_risk_level: None,
            min_retry_round: None,
            max_team_load: None,
        },
        MergeReworkRule {
            marker: "[[merge:test-conflict]]".to_string(),
            route_key: "test-conflict".to_string(),
            priority: 30,
            condition_mode: default_condition_mode(),
            required_risk_level: None,
            min_retry_round: None,
            max_team_load: None,
        },
        MergeReworkRule {
            marker: "[[merge:conflict]]".to_string(),
            route_key: "generic".to_string(),
            priority: 100,
            condition_mode: default_condition_mode(),
            required_risk_level: None,
            min_retry_round: None,
            max_team_load: None,
        },
    ]
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum ProjectStatus {
    Completed,
    NeedsHumanDecision,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProjectReport {
    pub goal: GoalContract,
    pub status: ProjectStatus,
    pub tasks: Vec<TaskReport>,
    pub merge: Option<MergeOutcome>,
    pub gates: Vec<GateOutcome>,
    pub trace: Vec<String>,
}
