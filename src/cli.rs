use std::path::PathBuf;

use anyhow::Result;
use clap::{Parser, Subcommand};

use crate::gitops::rollback_to_checkpoint;
use crate::guard::ExecutionGuard;
use crate::jobs::JobService;
use crate::runtime::bootstrap::registry_from_profile;
use crate::runtime::profile::RuntimeProfile;
use crate::runtime::project_runtime::ProjectRuntime;

#[derive(Debug, Parser)]
#[command(name = "orch-rs", version, about = "Rust AI Orchestrator CLI")]
struct Cli {
    #[arg(long, default_value = "data/orchestrator.db")]
    db: PathBuf,
    #[arg(long, default_value = "data")]
    data_dir: PathBuf,
    #[arg(long)]
    workflow_file: Option<PathBuf>,
    #[command(subcommand)]
    command: Commands,
}

#[derive(Debug, Subcommand)]
enum Commands {
    /// Submit an async job
    Submit {
        requirement: String,
        #[arg(long, default_value = "mvp")]
        workflow: String,
    },
    /// Process queued jobs
    Work {
        #[arg(long, default_value_t = 8)]
        limit: usize,
    },
    /// Show job status
    JobStatus { job_id: String },
    /// Show final job result
    Result { job_id: String },
    /// Resume a paused job after approvals
    Resume { job_id: String },
    /// Show persisted pipeline trace (job, decisions, discussions)
    Trace { pipeline_id: String },
    /// List pending human-gate decisions
    Pending {
        #[arg(long)]
        pipeline_id: Option<String>,
    },
    /// Approve a pending human-gate decision
    Approve { decision_id: String },
    /// Reject a pending human-gate decision
    Reject {
        decision_id: String,
        #[arg(long)]
        reason: Option<String>,
    },
    /// Convenience command: submit and run immediately
    Run {
        requirement: String,
        #[arg(long, default_value = "mvp")]
        workflow: String,
    },
    /// Roll back a workspace to a checkpoint commit
    Rollback { workdir: PathBuf, commit: String },
    /// Validate a shell command against hard guardrails
    GuardShell { command: String },
    /// Run enterprise-style agent team flow
    TeamRun {
        requirement: String,
        #[arg(long, default_value_t = 3)]
        max_parallel: usize,
        #[arg(long)]
        team_topology: Option<String>,
        #[arg(long)]
        max_parallel_teams: Option<usize>,
        #[arg(long)]
        enable_role_failover: bool,
        #[arg(long)]
        max_role_attempts: Option<usize>,
        #[arg(long)]
        profile_file: Option<PathBuf>,
        #[arg(long)]
        gate_policy: Option<String>,
        #[arg(long)]
        arbiter_policy: Option<String>,
        #[arg(long)]
        merge_policy: Option<String>,
        #[arg(long)]
        enable_merge_auto_rework: bool,
        #[arg(long)]
        max_merge_retries: Option<u32>,
    },
}

pub fn run() -> Result<()> {
    let cli = Cli::parse();
    let build_service =
        || JobService::new_with_workflow_file(&cli.db, &cli.data_dir, cli.workflow_file.as_deref());

    match cli.command {
        Commands::Submit {
            requirement,
            workflow,
        } => {
            let service = build_service()?;
            let job_id = service.submit(&requirement, &workflow)?;
            println!("{job_id}");
        }
        Commands::Work { limit } => {
            let service = build_service()?;
            let processed = service.process_queued(limit)?;
            println!("processed={processed}");
        }
        Commands::JobStatus { job_id } => {
            let service = build_service()?;
            let status = service.get_status(&job_id)?;
            if let Some(payload) = status {
                println!("{}", serde_json::to_string_pretty(&payload)?);
            } else {
                println!("Job not found.");
            }
        }
        Commands::Result { job_id } => {
            let service = build_service()?;
            let result = service.get_result(&job_id)?;
            if let Some(payload) = result {
                println!("{}", serde_json::to_string_pretty(&payload)?);
            } else {
                println!("Job not found.");
            }
        }
        Commands::Resume { job_id } => {
            let service = build_service()?;
            service.resume_job(&job_id)?;
            let result = service.get_result(&job_id)?;
            if let Some(payload) = result {
                println!("{}", serde_json::to_string_pretty(&payload)?);
            } else {
                println!("Job not found.");
            }
        }
        Commands::Trace { pipeline_id } => {
            let service = build_service()?;
            let trace = service.trace_pipeline(&pipeline_id)?;
            println!("{}", serde_json::to_string_pretty(&trace)?);
        }
        Commands::Pending { pipeline_id } => {
            let service = build_service()?;
            let pending = service.list_pending_decisions(pipeline_id.as_deref())?;
            if pending.is_empty() {
                println!("No pending decisions.");
            } else {
                println!("{}", serde_json::to_string_pretty(&pending)?);
            }
        }
        Commands::Approve { decision_id } => {
            let service = build_service()?;
            service.approve_decision(&decision_id)?;
            println!("approved={decision_id}");
        }
        Commands::Reject {
            decision_id,
            reason,
        } => {
            let service = build_service()?;
            service.reject_decision(&decision_id, reason.as_deref())?;
            println!("rejected={decision_id}");
        }
        Commands::Run {
            requirement,
            workflow,
        } => {
            let service = build_service()?;
            let job_id = service.submit(&requirement, &workflow)?;
            service.run_job(&job_id)?;
            let result = service.get_result(&job_id)?;
            if let Some(payload) = result {
                println!("{}", serde_json::to_string_pretty(&payload)?);
            } else {
                println!("Job not found.");
            }
        }
        Commands::Rollback { workdir, commit } => {
            rollback_to_checkpoint(&workdir, &commit)?;
            println!("rollback=ok");
        }
        Commands::GuardShell { command } => {
            let decision = ExecutionGuard::default().validate_shell(&command);
            println!("{}", serde_json::to_string_pretty(&decision)?);
        }
        Commands::TeamRun {
            requirement,
            max_parallel,
            team_topology,
            max_parallel_teams,
            enable_role_failover,
            max_role_attempts,
            profile_file,
            gate_policy,
            arbiter_policy,
            merge_policy,
            enable_merge_auto_rework,
            max_merge_retries,
        } => {
            let profile = RuntimeProfile::load(profile_file.as_deref())?
                .with_gate_policy(gate_policy)
                .with_arbiter_policy(arbiter_policy)
                .with_merge_policy(merge_policy)
                .with_merge_auto_rework(enable_merge_auto_rework)
                .with_max_merge_retries(max_merge_retries)
                .with_team_topology(team_topology)
                .with_max_parallel_teams(max_parallel_teams)
                .with_role_failover(enable_role_failover)
                .with_max_role_attempts(max_role_attempts);
            let runtime = ProjectRuntime::new(
                registry_from_profile(&profile)?,
                max_parallel,
                profile.max_parallel_teams,
                profile.merge_auto_rework,
                profile.max_merge_retries,
                profile.merge_rework_routes.clone(),
                profile.role_failover,
                profile.max_role_attempts,
            );
            let report = runtime.team_run(&requirement)?;
            println!("{}", serde_json::to_string_pretty(&report)?);
        }
    }

    Ok(())
}
