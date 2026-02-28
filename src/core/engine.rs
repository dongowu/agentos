use std::collections::HashSet;

use anyhow::{anyhow, Result};

use crate::core::models::{
    GateId, GateOutcome, GoalContract, ProjectReport, ProjectStatus, TaskReport,
};
use crate::core::scheduler::next_runnable;
use crate::core::trace::TraceLog;
use crate::plugins::PluginRegistry;

pub fn run_company_flow(
    requirement: &str,
    goal: GoalContract,
    max_parallel_tasks: usize,
    role_failover: bool,
    max_role_attempts: usize,
    plugins: &PluginRegistry,
) -> Result<ProjectReport> {
    let task_graph = plugins.team_strategy.build_task_graph(requirement, &goal);
    if task_graph.is_empty() {
        return Err(anyhow!("team strategy produced empty task graph"));
    }

    let mut completed = HashSet::new();
    let mut running = HashSet::new();
    let mut task_reports = Vec::new();
    let mut gate_outcomes = Vec::new();
    let mut trace = TraceLog::default();

    loop {
        let batch = next_runnable(&task_graph, &completed, &running, max_parallel_tasks);
        if batch.is_empty() {
            break;
        }

        let batch_names = batch.iter().map(|task| task.id.clone()).collect::<Vec<_>>();
        trace.push(format!("dispatch batch: {}", batch_names.join(", ")));

        for task in &batch {
            running.insert(task.id.clone());
        }

        for task in batch {
            let instances = plugins.role_provider.available_instances(&task.owner_role);
            let attempt_limit = if role_failover {
                max_role_attempts.max(1).min(instances.len().max(1))
            } else {
                1
            };

            let mut execution_result = None;
            let mut failure_messages = Vec::new();
            for instance_id in instances.iter().take(attempt_limit) {
                match plugins
                    .role_provider
                    .execute(&task.owner_role, instance_id, &task, &goal)
                {
                    Ok(execution) => {
                        execution_result = Some(execution);
                        if failure_messages.is_empty() {
                            trace.push(format!("task completed: {} by {}", task.id, instance_id));
                        } else {
                            trace.push(format!(
                                "task completed after failover: {} by {}",
                                task.id, instance_id
                            ));
                        }
                        break;
                    }
                    Err(err) => {
                        let message = format!(
                            "task {} failed on instance {}: {}",
                            task.id, instance_id, err
                        );
                        trace.push(message.clone());
                        failure_messages.push(message);
                    }
                }
            }

            let Some(execution) = execution_result else {
                running.remove(&task.id);
                task_reports.push(TaskReport {
                    task_id: task.id.clone(),
                    role: task.owner_role.clone(),
                    summary: format!(
                        "all role instances failed ({} attempts)",
                        failure_messages.len()
                    ),
                    risk_level: "high".to_string(),
                    artifacts: Vec::new(),
                });
                return Ok(ProjectReport {
                    goal,
                    status: ProjectStatus::NeedsHumanDecision,
                    tasks: task_reports,
                    gates: gate_outcomes,
                    trace: trace.into_entries(),
                });
            };

            let risk_level = plugins.risk_policy.classify(&execution);
            task_reports.push(TaskReport {
                task_id: task.id.clone(),
                role: format!("{}@{}", task.owner_role, execution.instance_id),
                summary: execution.summary,
                risk_level,
                artifacts: execution.artifacts,
            });
            running.remove(&task.id);
            completed.insert(task.id);
        }

        if completed.contains("intake") && !has_gate(&gate_outcomes, GateId::Intake) {
            let outcome = evaluate_gate(
                GateId::Intake,
                requirement,
                &task_reports,
                plugins,
                &mut trace,
            );
            if !outcome.approved {
                gate_outcomes.push(outcome);
                return Ok(ProjectReport {
                    goal,
                    status: ProjectStatus::NeedsHumanDecision,
                    tasks: task_reports,
                    gates: gate_outcomes,
                    trace: trace.into_entries(),
                });
            }
            gate_outcomes.push(outcome);
        }

        if completed.contains("design") && !has_gate(&gate_outcomes, GateId::Freeze) {
            let outcome = evaluate_gate(
                GateId::Freeze,
                requirement,
                &task_reports,
                plugins,
                &mut trace,
            );
            if !outcome.approved {
                gate_outcomes.push(outcome);
                return Ok(ProjectReport {
                    goal,
                    status: ProjectStatus::NeedsHumanDecision,
                    tasks: task_reports,
                    gates: gate_outcomes,
                    trace: trace.into_entries(),
                });
            }
            gate_outcomes.push(outcome);
        }

        if completed.contains("release_plan") && !has_gate(&gate_outcomes, GateId::Release) {
            let outcome = evaluate_gate(
                GateId::Release,
                requirement,
                &task_reports,
                plugins,
                &mut trace,
            );
            if !outcome.approved {
                gate_outcomes.push(outcome);
                return Ok(ProjectReport {
                    goal,
                    status: ProjectStatus::NeedsHumanDecision,
                    tasks: task_reports,
                    gates: gate_outcomes,
                    trace: trace.into_entries(),
                });
            }
            gate_outcomes.push(outcome);
        }
    }

    let closure = evaluate_gate(
        GateId::Closure,
        requirement,
        &task_reports,
        plugins,
        &mut trace,
    );
    let status = if closure.approved {
        ProjectStatus::Completed
    } else {
        ProjectStatus::NeedsHumanDecision
    };
    gate_outcomes.push(closure);

    Ok(ProjectReport {
        goal,
        status,
        tasks: task_reports,
        gates: gate_outcomes,
        trace: trace.into_entries(),
    })
}

fn has_gate(gates: &[GateOutcome], gate: GateId) -> bool {
    gates.iter().any(|outcome| outcome.gate == gate)
}

fn evaluate_gate(
    gate: GateId,
    requirement: &str,
    reports: &[TaskReport],
    plugins: &PluginRegistry,
    trace: &mut TraceLog,
) -> GateOutcome {
    let votes = plugins
        .gate_policy
        .evaluate(gate.clone(), reports, requirement);
    let unanimous = votes.iter().all(|vote| vote.approved);
    if unanimous {
        trace.push(format!("gate {:?} approved unanimously", gate));
        return GateOutcome {
            gate,
            approved: true,
            votes,
            arbitration_note: None,
            escalated_to_human: false,
        };
    }

    trace.push(format!("gate {:?} blocked, invoking arbiter", gate));
    let arbiter = plugins.arbiter_policy.resolve(gate.clone(), &votes);
    trace.push(format!("arbiter decision for {:?}: {}", gate, arbiter.note));
    GateOutcome {
        gate,
        approved: arbiter.approved,
        votes,
        arbitration_note: Some(arbiter.note),
        escalated_to_human: arbiter.escalated_to_human,
    }
}

#[cfg(test)]
mod tests {
    use super::run_company_flow;
    use crate::core::models::{GoalContract, ProjectStatus};
    use crate::runtime::bootstrap::registry_from_profile;
    use crate::runtime::profile::RuntimeProfile;

    #[test]
    fn company_flow_completes_by_default() {
        let plugins = registry_from_profile(&RuntimeProfile::default()).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_1".to_string(),
            objective: "ship feature".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow("ship feature", goal, 3, false, 2, &plugins).expect("report");
        assert_eq!(report.status, ProjectStatus::Completed);
        assert_eq!(report.gates.len(), 4);
        assert!(report.gates.iter().all(|gate| gate.approved));
    }

    #[test]
    fn company_flow_escalates_on_security_veto() {
        let plugins = registry_from_profile(&RuntimeProfile::default()).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_2".to_string(),
            objective: "ship feature".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature [[veto:security]]",
            goal,
            3,
            false,
            2,
            &plugins,
        )
        .expect("report");
        assert_eq!(report.status, ProjectStatus::NeedsHumanDecision);
        let release_gate = report
            .gates
            .iter()
            .find(|gate| gate.gate == crate::core::models::GateId::Release)
            .expect("release gate");
        assert!(!release_gate.approved);
        assert!(release_gate.escalated_to_human);
    }

    #[test]
    fn company_flow_retries_role_with_failover() {
        let plugins = registry_from_profile(&RuntimeProfile::default()).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_3".to_string(),
            objective: "ship feature [[failover:coder]]".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature [[failover:coder]]",
            goal,
            3,
            true,
            2,
            &plugins,
        )
        .expect("report");
        assert_eq!(report.status, ProjectStatus::Completed);
        assert!(report
            .trace
            .iter()
            .any(|line| line.contains("task completed after failover: implementation")));
    }
}
