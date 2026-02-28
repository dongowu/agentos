use serde::{Deserialize, Serialize};

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
