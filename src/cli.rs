use std::path::PathBuf;

use anyhow::Result;
use clap::{Parser, Subcommand};

use crate::gitops::rollback_to_checkpoint;
use crate::guard::ExecutionGuard;
use crate::jobs::JobService;

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
}

pub fn run() -> Result<()> {
    let cli = Cli::parse();
    let service =
        JobService::new_with_workflow_file(&cli.db, &cli.data_dir, cli.workflow_file.as_deref())?;

    match cli.command {
        Commands::Submit {
            requirement,
            workflow,
        } => {
            let job_id = service.submit(&requirement, &workflow)?;
            println!("{job_id}");
        }
        Commands::Work { limit } => {
            let processed = service.process_queued(limit)?;
            println!("processed={processed}");
        }
        Commands::JobStatus { job_id } => {
            let status = service.get_status(&job_id)?;
            if let Some(payload) = status {
                println!("{}", serde_json::to_string_pretty(&payload)?);
            } else {
                println!("Job not found.");
            }
        }
        Commands::Result { job_id } => {
            let result = service.get_result(&job_id)?;
            if let Some(payload) = result {
                println!("{}", serde_json::to_string_pretty(&payload)?);
            } else {
                println!("Job not found.");
            }
        }
        Commands::Resume { job_id } => {
            service.resume_job(&job_id)?;
            let result = service.get_result(&job_id)?;
            if let Some(payload) = result {
                println!("{}", serde_json::to_string_pretty(&payload)?);
            } else {
                println!("Job not found.");
            }
        }
        Commands::Trace { pipeline_id } => {
            let trace = service.trace_pipeline(&pipeline_id)?;
            println!("{}", serde_json::to_string_pretty(&trace)?);
        }
        Commands::Pending { pipeline_id } => {
            let pending = service.list_pending_decisions(pipeline_id.as_deref())?;
            if pending.is_empty() {
                println!("No pending decisions.");
            } else {
                println!("{}", serde_json::to_string_pretty(&pending)?);
            }
        }
        Commands::Approve { decision_id } => {
            service.approve_decision(&decision_id)?;
            println!("approved={decision_id}");
        }
        Commands::Reject {
            decision_id,
            reason,
        } => {
            service.reject_decision(&decision_id, reason.as_deref())?;
            println!("rejected={decision_id}");
        }
        Commands::Run {
            requirement,
            workflow,
        } => {
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
    }

    Ok(())
}
