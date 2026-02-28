use std::collections::HashMap;
use std::collections::HashSet;

use anyhow::{anyhow, Result};

use crate::core::models::{
    GateId, GateOutcome, GoalContract, MergeOutcome, MergeReworkRoute, ProjectReport,
    ProjectStatus, TaskNode, TaskReport,
};
use crate::core::scheduler::next_runnable;
use crate::core::trace::TraceLog;
use crate::plugins::PluginRegistry;

pub fn run_company_flow(
    requirement: &str,
    goal: GoalContract,
    max_parallel_tasks: usize,
    max_parallel_teams: usize,
    merge_auto_rework: bool,
    max_merge_retries: u32,
    merge_rework_routes: &HashMap<String, MergeReworkRoute>,
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
    let mut merge_outcome: Option<MergeOutcome> = None;
    let mut gate_outcomes = Vec::new();
    let mut trace = TraceLog::default();

    loop {
        let batch = next_runnable(&task_graph, &completed, &running, max_parallel_tasks);
        if batch.is_empty() {
            break;
        }

        let batch = limit_batch_by_team(batch, max_parallel_teams);

        let batch_names = batch.iter().map(|task| task.id.clone()).collect::<Vec<_>>();
        trace.push(format!("dispatch batch: {}", batch_names.join(", ")));
        let batch_teams = batch
            .iter()
            .map(|task| task.team_id.clone())
            .collect::<HashSet<_>>()
            .into_iter()
            .collect::<Vec<_>>();
        trace.push(format!("active teams: {}", batch_teams.join(", ")));

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
                    team_id: task.team_id.clone(),
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
                    merge: merge_outcome,
                    gates: gate_outcomes,
                    trace: trace.into_entries(),
                });
            };

            let risk_level = plugins.risk_policy.classify(&execution);
            task_reports.push(TaskReport {
                task_id: task.id.clone(),
                team_id: task.team_id.clone(),
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
                    merge: merge_outcome,
                    gates: gate_outcomes,
                    trace: trace.into_entries(),
                });
            }
            gate_outcomes.push(outcome);
        }

        if freeze_ready(&completed) && !has_gate(&gate_outcomes, GateId::Freeze) {
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
                    merge: merge_outcome,
                    gates: gate_outcomes,
                    trace: trace.into_entries(),
                });
            }
            gate_outcomes.push(outcome);
        }

        if merge_outcome.is_none() && merge_ready(&completed) {
            let outcome = evaluate_merge_with_auto_rework(
                requirement,
                &mut task_reports,
                plugins,
                &mut trace,
                merge_auto_rework,
                max_merge_retries,
                merge_rework_routes,
            );
            if !outcome.approved {
                merge_outcome = Some(outcome);
                return Ok(ProjectReport {
                    goal,
                    status: ProjectStatus::NeedsHumanDecision,
                    tasks: task_reports,
                    merge: merge_outcome,
                    gates: gate_outcomes,
                    trace: trace.into_entries(),
                });
            }
            merge_outcome = Some(outcome);
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
                    merge: merge_outcome,
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
        merge: merge_outcome,
        gates: gate_outcomes,
        trace: trace.into_entries(),
    })
}

fn has_gate(gates: &[GateOutcome], gate: GateId) -> bool {
    gates.iter().any(|outcome| outcome.gate == gate)
}

fn freeze_ready(completed: &HashSet<String>) -> bool {
    completed.contains("design")
        || (completed.contains("platform_design") && completed.contains("feature_design"))
}

fn merge_ready(completed: &HashSet<String>) -> bool {
    completed.contains("implementation")
        || (completed.contains("platform_impl") && completed.contains("feature_impl"))
}

fn limit_batch_by_team(batch: Vec<TaskNode>, max_parallel_teams: usize) -> Vec<TaskNode> {
    if max_parallel_teams == 0 {
        return Vec::new();
    }
    let mut selected = Vec::new();
    let mut teams = HashSet::new();
    for task in batch {
        if teams.contains(&task.team_id) || teams.len() < max_parallel_teams {
            teams.insert(task.team_id.clone());
            selected.push(task);
        }
    }
    selected
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

fn evaluate_merge(
    requirement: &str,
    reports: &[TaskReport],
    plugins: &PluginRegistry,
    trace: &mut TraceLog,
) -> MergeOutcome {
    let outcome = plugins.merge_policy.merge(reports, requirement);
    if outcome.approved {
        trace.push(format!(
            "merge approved (attempts={}): {}",
            outcome.attempts, outcome.note
        ));
    } else {
        trace.push(format!(
            "merge blocked (attempts={}): {}",
            outcome.attempts, outcome.note
        ));
    }
    outcome
}

fn evaluate_merge_with_auto_rework(
    requirement: &str,
    reports: &mut Vec<TaskReport>,
    plugins: &PluginRegistry,
    trace: &mut TraceLog,
    merge_auto_rework: bool,
    max_merge_retries: u32,
    merge_rework_routes: &HashMap<String, MergeReworkRoute>,
) -> MergeOutcome {
    let mut outcome = evaluate_merge(requirement, reports, plugins, trace);
    if outcome.approved || !merge_auto_rework {
        return outcome;
    }

    let route = detect_merge_rework_route(requirement, merge_rework_routes);
    let max_retries = max_merge_retries.max(1);
    for retry in 1..=max_retries {
        trace.push(format!(
            "merge auto-rework round {} ({}) : rollback to last checkpoint and regenerate conflicted outputs",
            retry,
            route.route_name
        ));
        reports.push(TaskReport {
            task_id: format!("merge_rework_{}_{}", route.task_suffix, retry),
            team_id: route.team_id.clone(),
            role: route.role.clone(),
            summary: format!(
                "{} executed merge rework round {} for cross-team convergence",
                route.actor_summary, retry
            ),
            risk_level: "medium".to_string(),
            artifacts: vec![format!(
                "artifacts/merge/{}/rework-round-{}.md",
                route.task_suffix, retry
            )],
        });

        outcome = evaluate_merge(requirement, reports, plugins, trace);
        if outcome.approved {
            return outcome;
        }
    }

    outcome
}

fn detect_merge_rework_route(
    requirement: &str,
    routes: &HashMap<String, MergeReworkRoute>,
) -> MergeReworkRoute {
    let key = if requirement.contains("[[merge:code-conflict]]") {
        "code-conflict"
    } else if requirement.contains("[[merge:api-conflict]]") {
        "api-conflict"
    } else if requirement.contains("[[merge:test-conflict]]") {
        "test-conflict"
    } else {
        "generic"
    };

    routes
        .get(key)
        .cloned()
        .or_else(|| routes.get("generic").cloned())
        .unwrap_or(MergeReworkRoute {
            route_name: "generic".to_string(),
            task_suffix: "generic".to_string(),
            team_id: "program_board".to_string(),
            role: "supervisor@supervisor.primary".to_string(),
            actor_summary: "supervisor".to_string(),
        })
}

#[cfg(test)]
mod tests {
    use super::run_company_flow;
    use crate::core::models::{GoalContract, ProjectStatus};
    use crate::runtime::bootstrap::registry_from_profile;
    use crate::runtime::profile::RuntimeProfile;

    #[test]
    fn company_flow_completes_by_default() {
        let profile = RuntimeProfile::default();
        let plugins = registry_from_profile(&profile).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_1".to_string(),
            objective: "ship feature".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature",
            goal,
            3,
            1,
            false,
            1,
            &profile.merge_rework_routes,
            false,
            2,
            &plugins,
        )
        .expect("report");
        assert_eq!(report.status, ProjectStatus::Completed);
        assert_eq!(report.gates.len(), 4);
        assert!(report.gates.iter().all(|gate| gate.approved));
    }

    #[test]
    fn company_flow_escalates_on_security_veto() {
        let profile = RuntimeProfile::default();
        let plugins = registry_from_profile(&profile).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_2".to_string(),
            objective: "ship feature".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature [[veto:security]]",
            goal,
            3,
            1,
            false,
            1,
            &profile.merge_rework_routes,
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
        let profile = RuntimeProfile::default();
        let plugins = registry_from_profile(&profile).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_3".to_string(),
            objective: "ship feature [[failover:coder]]".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature [[failover:coder]]",
            goal,
            3,
            1,
            false,
            1,
            &profile.merge_rework_routes,
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

    #[test]
    fn company_flow_multi_team_topology_dispatches_parallel_teams() {
        let mut profile = RuntimeProfile::default();
        profile.team_topology = "multi".to_string();
        let plugins = registry_from_profile(&profile).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_4".to_string(),
            objective: "ship feature".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature",
            goal,
            4,
            2,
            false,
            1,
            &profile.merge_rework_routes,
            false,
            2,
            &plugins,
        )
        .expect("report");
        assert_eq!(report.status, ProjectStatus::Completed);
        assert!(report
            .trace
            .iter()
            .any(|line| line.contains("dispatch batch: platform_design, feature_design")));
        assert!(report
            .tasks
            .iter()
            .any(|task| task.team_id == "platform_team"));
        assert!(report
            .tasks
            .iter()
            .any(|task| task.team_id == "feature_team"));
    }

    #[test]
    fn company_flow_blocks_when_strict_merge_conflict_persists() {
        let mut profile = RuntimeProfile::default();
        profile.team_topology = "multi".to_string();
        profile.merge_policy = "strict".to_string();
        let plugins = registry_from_profile(&profile).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_5".to_string(),
            objective: "ship feature [[merge:conflict]]".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature [[merge:conflict]]",
            goal,
            4,
            2,
            false,
            1,
            &profile.merge_rework_routes,
            false,
            2,
            &plugins,
        )
        .expect("report");
        assert_eq!(report.status, ProjectStatus::NeedsHumanDecision);
        let merge = report.merge.expect("merge outcome");
        assert!(!merge.approved);
        assert!(merge.escalated_to_human);
    }

    #[test]
    fn company_flow_allows_retry_successful_merge() {
        let mut profile = RuntimeProfile::default();
        profile.team_topology = "multi".to_string();
        profile.merge_policy = "strict".to_string();
        let plugins = registry_from_profile(&profile).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_6".to_string(),
            objective: "ship feature [[merge:conflict]] [[merge:retry-ok]]".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature [[merge:conflict]] [[merge:retry-ok]]",
            goal,
            4,
            2,
            false,
            1,
            &profile.merge_rework_routes,
            false,
            2,
            &plugins,
        )
        .expect("report");
        assert_eq!(report.status, ProjectStatus::Completed);
        let merge = report.merge.expect("merge outcome");
        assert!(merge.approved);
        assert_eq!(merge.attempts, 2);
    }

    #[test]
    fn company_flow_auto_rework_recovers_merge_conflict() {
        let mut profile = RuntimeProfile::default();
        profile.team_topology = "multi".to_string();
        profile.merge_policy = "strict".to_string();
        let plugins = registry_from_profile(&profile).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_7".to_string(),
            objective: "ship feature [[merge:conflict]]".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature [[merge:conflict]]",
            goal,
            4,
            2,
            true,
            2,
            &profile.merge_rework_routes,
            false,
            2,
            &plugins,
        )
        .expect("report");
        assert_eq!(report.status, ProjectStatus::Completed);
        let merge = report.merge.expect("merge outcome");
        assert!(merge.approved);
        assert!(report
            .trace
            .iter()
            .any(|line| line.contains("merge auto-rework round 1")));
    }

    #[test]
    fn company_flow_routes_api_conflict_rework_to_feature_team() {
        let mut profile = RuntimeProfile::default();
        profile.team_topology = "multi".to_string();
        profile.merge_policy = "strict".to_string();
        let plugins = registry_from_profile(&profile).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_8".to_string(),
            objective: "ship feature [[merge:api-conflict]]".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature [[merge:api-conflict]]",
            goal,
            4,
            2,
            true,
            2,
            &profile.merge_rework_routes,
            false,
            2,
            &plugins,
        )
        .expect("report");
        assert_eq!(report.status, ProjectStatus::Completed);
        assert!(report
            .tasks
            .iter()
            .any(|task| task.task_id == "merge_rework_api_1" && task.team_id == "feature_team"));
    }

    #[test]
    fn company_flow_respects_configured_merge_route_overrides() {
        let mut profile = RuntimeProfile::default();
        profile.team_topology = "multi".to_string();
        profile.merge_policy = "strict".to_string();
        if let Some(route) = profile.merge_rework_routes.get_mut("api-conflict") {
            route.team_id = "qa_team".to_string();
            route.role = "tester@tester.primary".to_string();
            route.actor_summary = "qa override lead".to_string();
        }
        let plugins = registry_from_profile(&profile).expect("plugins");
        let goal = GoalContract {
            goal_id: "goal_test_9".to_string(),
            objective: "ship feature [[merge:api-conflict]]".to_string(),
            acceptance_criteria: vec!["tests pass".to_string()],
        };

        let report = run_company_flow(
            "ship feature [[merge:api-conflict]]",
            goal,
            4,
            2,
            true,
            2,
            &profile.merge_rework_routes,
            false,
            2,
            &plugins,
        )
        .expect("report");

        assert_eq!(report.status, ProjectStatus::Completed);
        assert!(report.tasks.iter().any(|task| {
            task.task_id == "merge_rework_api_1"
                && task.team_id == "qa_team"
                && task.role == "tester@tester.primary"
        }));
    }
}
