use std::collections::HashMap;

use anyhow::anyhow;
use anyhow::Result;

use crate::core::models::{
    Department, GateId, GateVote, GoalContract, MergeOutcome, TaskNode, TaskReport,
};
use crate::plugins::{
    ArbiterDecision, ArbiterPolicy, GatePolicy, MergePolicy, RiskPolicy, RoleExecution,
    RoleProvider, TeamStrategy,
};

pub struct BuiltinRoleProvider {
    role_instances: HashMap<String, Vec<String>>,
}

impl BuiltinRoleProvider {
    pub fn from_overrides(overrides: HashMap<String, Vec<String>>) -> Self {
        let mut role_instances = HashMap::from([
            (
                "product_lead".to_string(),
                vec!["product_lead.primary".to_string()],
            ),
            (
                "architect".to_string(),
                vec!["architect.primary".to_string()],
            ),
            (
                "coder".to_string(),
                vec!["coder.primary".to_string(), "coder.backup".to_string()],
            ),
            (
                "tester".to_string(),
                vec!["tester.primary".to_string(), "tester.backup".to_string()],
            ),
            (
                "security_lead".to_string(),
                vec!["security_lead.primary".to_string()],
            ),
            (
                "release_manager".to_string(),
                vec!["release_manager.primary".to_string()],
            ),
        ]);
        for (role, instances) in overrides {
            role_instances.insert(role, instances);
        }
        Self { role_instances }
    }
}

impl RoleProvider for BuiltinRoleProvider {
    fn available_instances(&self, role: &str) -> Vec<String> {
        self.role_instances
            .get(role)
            .cloned()
            .unwrap_or_else(|| vec![format!("{}.primary", role)])
    }

    fn execute(
        &self,
        role: &str,
        instance_id: &str,
        task: &TaskNode,
        goal: &GoalContract,
    ) -> Result<RoleExecution> {
        let failover_marker = format!("[[failover:{}]]", role);
        if goal.objective.contains(&failover_marker) && instance_id.ends_with(".primary") {
            return Err(anyhow!(
                "simulated failure on {} for role {}",
                instance_id,
                role
            ));
        }
        let summary = format!(
            "{} ({}) completed '{}' for goal {}",
            role, instance_id, task.title, goal.goal_id
        );
        let artifacts = vec![format!("artifacts/{}/{}.md", task.id, instance_id)];
        Ok(RoleExecution {
            instance_id: instance_id.to_string(),
            summary,
            artifacts,
        })
    }
}

pub struct BuiltinTeamStrategy {
    topology: String,
}

impl BuiltinTeamStrategy {
    pub fn new(topology: String) -> Self {
        Self { topology }
    }

    fn single_team_graph(requirement: &str) -> Vec<TaskNode> {
        let team_id = "delivery_core".to_string();
        vec![
            TaskNode {
                id: "intake".to_string(),
                team_id: team_id.clone(),
                title: format!("Intake and scope requirement: {}", requirement),
                owner_role: "product_lead".to_string(),
                department: Department::Product,
                depends_on: Vec::new(),
            },
            TaskNode {
                id: "design".to_string(),
                team_id: team_id.clone(),
                title: "Create technical design".to_string(),
                owner_role: "architect".to_string(),
                department: Department::Engineering,
                depends_on: vec!["intake".to_string()],
            },
            TaskNode {
                id: "implementation".to_string(),
                team_id: team_id.clone(),
                title: "Implement delivery plan".to_string(),
                owner_role: "coder".to_string(),
                department: Department::Engineering,
                depends_on: vec!["design".to_string()],
            },
            TaskNode {
                id: "qa_validation".to_string(),
                team_id: team_id.clone(),
                title: "Run functional and regression tests".to_string(),
                owner_role: "tester".to_string(),
                department: Department::Qa,
                depends_on: vec!["implementation".to_string()],
            },
            TaskNode {
                id: "security_review".to_string(),
                team_id: team_id.clone(),
                title: "Review security and compliance risk".to_string(),
                owner_role: "security_lead".to_string(),
                department: Department::Security,
                depends_on: vec!["implementation".to_string()],
            },
            TaskNode {
                id: "release_plan".to_string(),
                team_id: team_id.clone(),
                title: "Prepare release checklist and rollback".to_string(),
                owner_role: "release_manager".to_string(),
                department: Department::Ops,
                depends_on: vec!["qa_validation".to_string(), "security_review".to_string()],
            },
            TaskNode {
                id: "retrospective".to_string(),
                team_id,
                title: "Close project and capture lessons learned".to_string(),
                owner_role: "product_lead".to_string(),
                department: Department::Product,
                depends_on: vec!["release_plan".to_string()],
            },
        ]
    }

    fn multi_team_graph(requirement: &str) -> Vec<TaskNode> {
        vec![
            TaskNode {
                id: "intake".to_string(),
                team_id: "program_board".to_string(),
                title: format!("Intake and scope requirement: {}", requirement),
                owner_role: "product_lead".to_string(),
                department: Department::Product,
                depends_on: Vec::new(),
            },
            TaskNode {
                id: "platform_design".to_string(),
                team_id: "platform_team".to_string(),
                title: "Platform team creates shared technical design".to_string(),
                owner_role: "architect".to_string(),
                department: Department::Engineering,
                depends_on: vec!["intake".to_string()],
            },
            TaskNode {
                id: "feature_design".to_string(),
                team_id: "feature_team".to_string(),
                title: "Feature team defines delivery increments".to_string(),
                owner_role: "architect".to_string(),
                department: Department::Engineering,
                depends_on: vec!["intake".to_string()],
            },
            TaskNode {
                id: "platform_impl".to_string(),
                team_id: "platform_team".to_string(),
                title: "Platform team implementation".to_string(),
                owner_role: "coder".to_string(),
                department: Department::Engineering,
                depends_on: vec!["platform_design".to_string()],
            },
            TaskNode {
                id: "feature_impl".to_string(),
                team_id: "feature_team".to_string(),
                title: "Feature team implementation".to_string(),
                owner_role: "coder".to_string(),
                department: Department::Engineering,
                depends_on: vec!["feature_design".to_string()],
            },
            TaskNode {
                id: "platform_qa".to_string(),
                team_id: "qa_team".to_string(),
                title: "QA validates platform deliverables".to_string(),
                owner_role: "tester".to_string(),
                department: Department::Qa,
                depends_on: vec!["platform_impl".to_string()],
            },
            TaskNode {
                id: "feature_qa".to_string(),
                team_id: "qa_team".to_string(),
                title: "QA validates feature deliverables".to_string(),
                owner_role: "tester".to_string(),
                department: Department::Qa,
                depends_on: vec!["feature_impl".to_string()],
            },
            TaskNode {
                id: "security_review".to_string(),
                team_id: "security_team".to_string(),
                title: "Review security and compliance risk".to_string(),
                owner_role: "security_lead".to_string(),
                department: Department::Security,
                depends_on: vec!["platform_impl".to_string(), "feature_impl".to_string()],
            },
            TaskNode {
                id: "release_plan".to_string(),
                team_id: "release_team".to_string(),
                title: "Prepare release checklist and rollback".to_string(),
                owner_role: "release_manager".to_string(),
                department: Department::Ops,
                depends_on: vec![
                    "platform_qa".to_string(),
                    "feature_qa".to_string(),
                    "security_review".to_string(),
                ],
            },
            TaskNode {
                id: "retrospective".to_string(),
                team_id: "program_board".to_string(),
                title: "Close project and capture lessons learned".to_string(),
                owner_role: "product_lead".to_string(),
                department: Department::Product,
                depends_on: vec!["release_plan".to_string()],
            },
        ]
    }
}

impl TeamStrategy for BuiltinTeamStrategy {
    fn build_task_graph(&self, requirement: &str, _goal: &GoalContract) -> Vec<TaskNode> {
        if self.topology == "multi" {
            Self::multi_team_graph(requirement)
        } else {
            Self::single_team_graph(requirement)
        }
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

pub struct MajorityGatePolicy;

impl GatePolicy for MajorityGatePolicy {
    fn evaluate(&self, gate: GateId, reports: &[TaskReport], requirement: &str) -> Vec<GateVote> {
        let base = UnanimousGatePolicy;
        let mut votes = base.evaluate(gate.clone(), reports, requirement);

        // Majority mode allows QA soft objections to proceed when three or more
        // departments agree, while keeping explicit security veto markers intact.
        if gate == GateId::Freeze && reports.iter().any(|report| report.risk_level == "high") {
            if let Some(qa) = votes
                .iter_mut()
                .find(|vote| vote.department == Department::Qa)
            {
                qa.reason = "qa warns of elevated risk but defers to board majority".to_string();
                qa.approved = false;
            }
            if let Some(engineering) = votes
                .iter_mut()
                .find(|vote| vote.department == Department::Engineering)
            {
                engineering.reason = "engineering accepts mitigation plan".to_string();
                engineering.approved = true;
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

pub struct EscalateImmediatelyArbiter;

impl ArbiterPolicy for EscalateImmediatelyArbiter {
    fn resolve(&self, gate: GateId, votes: &[GateVote]) -> ArbiterDecision {
        if votes.iter().all(|vote| vote.approved) {
            return ArbiterDecision {
                approved: true,
                note: format!("Arbiter confirms {:?} gate pass", gate),
                escalated_to_human: false,
            };
        }

        ArbiterDecision {
            approved: false,
            note: format!(
                "Arbiter escalates {:?} immediately due to policy 'immediate_escalation'",
                gate
            ),
            escalated_to_human: true,
        }
    }
}

pub struct StrictMergePolicy;

impl MergePolicy for StrictMergePolicy {
    fn merge(&self, reports: &[TaskReport], requirement: &str) -> MergeOutcome {
        let has_cross_team_reports = reports.iter().any(|r| r.team_id == "platform_team")
            && reports.iter().any(|r| r.team_id == "feature_team");
        if !has_cross_team_reports {
            return MergeOutcome {
                approved: true,
                attempts: 1,
                note: "single-team delivery, no cross-team merge required".to_string(),
                escalated_to_human: false,
            };
        }

        if has_merge_conflict_marker(requirement) {
            if has_expected_rework_evidence(reports, requirement) {
                return MergeOutcome {
                    approved: true,
                    attempts: 2,
                    note: "merge conflict resolved after automated rework".to_string(),
                    escalated_to_human: false,
                };
            }
            if requirement.contains("[[merge:retry-ok]]") {
                return MergeOutcome {
                    approved: true,
                    attempts: 2,
                    note: "merge conflict resolved on retry with supervisor guidance".to_string(),
                    escalated_to_human: false,
                };
            }
            return MergeOutcome {
                approved: false,
                attempts: 2,
                note: "merge conflict persists after retry, escalation required".to_string(),
                escalated_to_human: true,
            };
        }

        MergeOutcome {
            approved: true,
            attempts: 1,
            note: "cross-team artifacts merged successfully".to_string(),
            escalated_to_human: false,
        }
    }
}

fn has_merge_conflict_marker(requirement: &str) -> bool {
    requirement.contains("[[merge:conflict]]")
        || requirement.contains("[[merge:code-conflict]]")
        || requirement.contains("[[merge:api-conflict]]")
        || requirement.contains("[[merge:test-conflict]]")
}

fn has_expected_rework_evidence(reports: &[TaskReport], requirement: &str) -> bool {
    let expected_suffix = if requirement.contains("[[merge:code-conflict]]") {
        "code"
    } else if requirement.contains("[[merge:api-conflict]]") {
        "api"
    } else if requirement.contains("[[merge:test-conflict]]") {
        "test"
    } else {
        "generic"
    };

    reports.iter().any(|report| {
        report
            .task_id
            .starts_with(&format!("merge_rework_{}", expected_suffix))
    })
}

pub struct FastMergePolicy;

impl MergePolicy for FastMergePolicy {
    fn merge(&self, reports: &[TaskReport], requirement: &str) -> MergeOutcome {
        let has_cross_team_reports = reports.iter().any(|r| r.team_id == "platform_team")
            && reports.iter().any(|r| r.team_id == "feature_team");
        if !has_cross_team_reports {
            return MergeOutcome {
                approved: true,
                attempts: 1,
                note: "single-team delivery, no cross-team merge required".to_string(),
                escalated_to_human: false,
            };
        }

        if requirement.contains("[[merge:block]]") {
            return MergeOutcome {
                approved: false,
                attempts: 1,
                note: "fast merge policy blocked by explicit requirement marker".to_string(),
                escalated_to_human: true,
            };
        }

        MergeOutcome {
            approved: true,
            attempts: 1,
            note: "fast merge accepted artifacts without retry".to_string(),
            escalated_to_human: false,
        }
    }
}
