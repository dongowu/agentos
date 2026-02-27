use anyhow::Result;

use crate::core::models::{Department, GateId, GateVote, GoalContract, TaskNode, TaskReport};
use crate::plugins::{
    ArbiterDecision, ArbiterPolicy, GatePolicy, RiskPolicy, RoleExecution, RoleProvider,
    TeamStrategy,
};

pub struct BuiltinRoleProvider;

impl RoleProvider for BuiltinRoleProvider {
    fn execute(&self, role: &str, task: &TaskNode, goal: &GoalContract) -> Result<RoleExecution> {
        let summary = format!(
            "{} completed '{}' for goal {}",
            role, task.title, goal.goal_id
        );
        let artifacts = vec![format!("artifacts/{}/{}.md", task.id, role)];
        Ok(RoleExecution { summary, artifacts })
    }
}

pub struct BuiltinTeamStrategy;

impl TeamStrategy for BuiltinTeamStrategy {
    fn build_task_graph(&self, requirement: &str, _goal: &GoalContract) -> Vec<TaskNode> {
        vec![
            TaskNode {
                id: "intake".to_string(),
                title: format!("Intake and scope requirement: {}", requirement),
                owner_role: "product_lead".to_string(),
                department: Department::Product,
                depends_on: Vec::new(),
            },
            TaskNode {
                id: "design".to_string(),
                title: "Create technical design".to_string(),
                owner_role: "architect".to_string(),
                department: Department::Engineering,
                depends_on: vec!["intake".to_string()],
            },
            TaskNode {
                id: "implementation".to_string(),
                title: "Implement delivery plan".to_string(),
                owner_role: "coder".to_string(),
                department: Department::Engineering,
                depends_on: vec!["design".to_string()],
            },
            TaskNode {
                id: "qa_validation".to_string(),
                title: "Run functional and regression tests".to_string(),
                owner_role: "tester".to_string(),
                department: Department::Qa,
                depends_on: vec!["implementation".to_string()],
            },
            TaskNode {
                id: "security_review".to_string(),
                title: "Review security and compliance risk".to_string(),
                owner_role: "security_lead".to_string(),
                department: Department::Security,
                depends_on: vec!["implementation".to_string()],
            },
            TaskNode {
                id: "release_plan".to_string(),
                title: "Prepare release checklist and rollback".to_string(),
                owner_role: "release_manager".to_string(),
                department: Department::Ops,
                depends_on: vec!["qa_validation".to_string(), "security_review".to_string()],
            },
            TaskNode {
                id: "retrospective".to_string(),
                title: "Close project and capture lessons learned".to_string(),
                owner_role: "product_lead".to_string(),
                department: Department::Product,
                depends_on: vec!["release_plan".to_string()],
            },
        ]
    }
}

pub struct BuiltinRiskPolicy;

impl RiskPolicy for BuiltinRiskPolicy {
    fn classify(&self, execution: &RoleExecution) -> String {
        if execution.summary.contains("release") || execution.summary.contains("security") {
            "medium".to_string()
        } else {
            "low".to_string()
        }
    }
}

pub struct UnanimousGatePolicy;

impl GatePolicy for UnanimousGatePolicy {
    fn evaluate(&self, gate: GateId, reports: &[TaskReport], requirement: &str) -> Vec<GateVote> {
        let mut votes = vec![
            GateVote {
                department: Department::Product,
                approved: true,
                reason: "goal remains aligned".to_string(),
            },
            GateVote {
                department: Department::Engineering,
                approved: true,
                reason: "implementation quality acceptable".to_string(),
            },
            GateVote {
                department: Department::Qa,
                approved: true,
                reason: "test coverage is sufficient".to_string(),
            },
            GateVote {
                department: Department::Security,
                approved: true,
                reason: "no blocking security findings".to_string(),
            },
            GateVote {
                department: Department::Ops,
                approved: true,
                reason: "release readiness confirmed".to_string(),
            },
        ];

        if gate == GateId::Release && requirement.contains("[[veto:security]]") {
            if let Some(security) = votes
                .iter_mut()
                .find(|vote| vote.department == Department::Security)
            {
                security.approved = false;
                security.reason =
                    "security policy veto triggered by requirement marker".to_string();
            }
        }

        if gate == GateId::Freeze && reports.iter().any(|report| report.risk_level == "high") {
            if let Some(qa) = votes
                .iter_mut()
                .find(|vote| vote.department == Department::Qa)
            {
                qa.approved = false;
                qa.reason = "high risk detected before scope freeze".to_string();
            }
        }

        votes
    }
}

pub struct TwoRoundArbiter;

impl ArbiterPolicy for TwoRoundArbiter {
    fn resolve(&self, gate: GateId, votes: &[GateVote]) -> ArbiterDecision {
        let blocking_departments: Vec<String> = votes
            .iter()
            .filter(|vote| !vote.approved)
            .map(|vote| format!("{:?}", vote.department))
            .collect();

        if blocking_departments.is_empty() {
            return ArbiterDecision {
                approved: true,
                note: format!("Arbiter confirms {:?} gate pass", gate),
                escalated_to_human: false,
            };
        }

        ArbiterDecision {
            approved: false,
            note: format!(
                "Arbiter attempted 2 rounds, still blocked by: {}",
                blocking_departments.join(", ")
            ),
            escalated_to_human: true,
        }
    }
}
