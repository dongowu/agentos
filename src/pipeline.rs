use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use chrono::Utc;
use regex::Regex;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::gitops::{create_checkpoint, rollback_to_checkpoint};
use crate::guard::ExecutionGuard;
use crate::messaging::{ConvergenceEngine, ConversationStatus, MessageBus};
use crate::rag::{RagDocument, RagRetriever, RagSnippet};
use crate::workflow_config::{StageDefinition, StageOutputSpec, WorkflowCatalog};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArtifactOutput {
    pub stage: String,
    pub kind: String,
    pub name: String,
    pub file_path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PipelineResult {
    pub pipeline_id: String,
    pub pipeline_status: String,
    pub outputs: Vec<ArtifactOutput>,
    pub discussions: Vec<DiscussionRecord>,
    pub last_stable_commit: Option<String>,
    pub run_state: RunState,
    pub working_dir: String,
    pub error: Option<String>,
    pub paused_stage: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DecisionRecord {
    pub round: u32,
    pub decision: String,
    pub reason: String,
    pub risk_score: f64,
    pub timestamp: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RunState {
    pub round: u32,
    pub max_rounds: u32,
    pub last_stable_commit: Option<String>,
    pub failure_pattern: Vec<String>,
    pub risk_score: f64,
    pub decision_history: Vec<DecisionRecord>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiscussionRecord {
    pub conversation_id: String,
    pub stage_id: String,
    pub topic: String,
    pub status: String,
    pub round_count: u32,
    pub max_rounds: u32,
    pub participants: Vec<String>,
    pub created_at: String,
    pub messages: Vec<DiscussionMessageRecord>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiscussionMessageRecord {
    pub id: String,
    pub round: u32,
    pub from_role: String,
    pub message_type: String,
    pub body: String,
    pub timestamp: String,
}

#[derive(Debug, Clone)]
struct DiscussionRun {
    outputs: Vec<ArtifactOutput>,
    status: ConversationStatus,
    record: DiscussionRecord,
}

#[derive(Debug, Clone)]
pub struct ResumeContext {
    pub pipeline_id: String,
    pub paused_stage: Option<String>,
    pub previous_outputs: Vec<ArtifactOutput>,
    pub previous_error: Option<String>,
}

impl RunState {
    fn new(max_rounds: u32) -> Self {
        Self {
            round: 0,
            max_rounds,
            last_stable_commit: None,
            failure_pattern: Vec::new(),
            risk_score: 0.0,
            decision_history: Vec::new(),
        }
    }
}

#[allow(dead_code)]
pub fn run_workflow(
    requirement: &str,
    workflow_id: &str,
    data_dir: &Path,
) -> Result<PipelineResult> {
    let catalog = WorkflowCatalog::load_or_default(None)?;
    run_workflow_with_catalog(requirement, workflow_id, data_dir, &catalog)
}

pub fn run_workflow_with_catalog(
    requirement: &str,
    workflow_id: &str,
    data_dir: &Path,
    catalog: &WorkflowCatalog,
) -> Result<PipelineResult> {
    execute_workflow(requirement, workflow_id, data_dir, catalog, None)
}

pub fn resume_workflow_with_catalog(
    requirement: &str,
    workflow_id: &str,
    data_dir: &Path,
    catalog: &WorkflowCatalog,
    resume: ResumeContext,
) -> Result<PipelineResult> {
    execute_workflow(requirement, workflow_id, data_dir, catalog, Some(resume))
}

fn execute_workflow(
    requirement: &str,
    workflow_id: &str,
    data_dir: &Path,
    catalog: &WorkflowCatalog,
    resume: Option<ResumeContext>,
) -> Result<PipelineResult> {
    let workflow = catalog
        .get(workflow_id)
        .with_context(|| format!("workflow \"{}\" not found", workflow_id))?;
    let pipeline_id = resume
        .as_ref()
        .map(|ctx| ctx.pipeline_id.clone())
        .unwrap_or_else(|| format!("pipe_{}", Uuid::new_v4().simple()));
    let working_dir = data_dir.join("worktrees").join(&pipeline_id);
    fs::create_dir_all(&working_dir)
        .with_context(|| format!("failed to create workdir {}", working_dir.display()))?;
    let mut run_state = RunState::new(6);
    let checkpoint_message = if resume.is_some() {
        "orchestrator-rs: resume checkpoint"
    } else {
        "orchestrator-rs: initial checkpoint"
    };
    let initial = create_checkpoint(&working_dir, checkpoint_message)
        .context("failed to create workflow checkpoint")?;
    run_state.last_stable_commit = Some(initial.commit.clone());

    let stage_index_map = build_stage_index_map(&workflow.stages);
    let mut outputs = if let Some(ctx) = resume.as_ref() {
        trim_outputs_for_resume(
            ctx.previous_outputs.clone(),
            &stage_index_map,
            ctx.paused_stage.as_deref(),
            ctx.previous_error.as_deref(),
        )
    } else {
        Vec::new()
    };
    let mut discussions = Vec::new();
    let mut outputs_by_stage = group_outputs_by_stage(&outputs);
    let mut pipeline_status = "completed".to_string();
    let mut error: Option<String> = None;
    let mut paused_stage: Option<String> = None;
    let autonomy_enabled = workflow_id == "autonomy";
    let mut forced = if autonomy_enabled {
        parse_forced_decisions(requirement)
    } else {
        Vec::new()
    };
    let danger = autonomy_enabled && has_danger_marker(requirement);
    let auto_approve_human_gate = requirement.contains("[[approve:all]]");
    let implementation_index = workflow
        .stages
        .iter()
        .position(|s| s.id == "implementation")
        .unwrap_or(0);

    let mut stage_index = resume_start_index(
        &workflow.stages,
        resume.as_ref().and_then(|ctx| ctx.paused_stage.as_deref()),
        resume
            .as_ref()
            .and_then(|ctx| ctx.previous_error.as_deref()),
    );
    while stage_index < workflow.stages.len() {
        let stage = &workflow.stages[stage_index];
        if autonomy_enabled && stage.id == "implementation" {
            run_state.round += 1;
        }

        let stage_inputs = collect_stage_inputs(stage, &outputs_by_stage);
        let stage_rag = build_rag_snippets(
            &stage.id,
            requirement,
            &working_dir,
            &stage_inputs,
            &run_state.failure_pattern,
        )?;

        let mut stage_outputs = Vec::new();
        if !stage.collaborators.is_empty() {
            let discussion = run_stage_discussion(
                &pipeline_id,
                stage,
                requirement,
                &working_dir,
                &stage_inputs,
            )?;
            stage_outputs.extend(discussion.outputs);
            discussions.push(discussion.record);
            if discussion.status == ConversationStatus::Escalated {
                pipeline_status = "paused".to_string();
                paused_stage = Some(stage.id.clone());
                error = Some(format!("discussion escalated at stage {}", stage.id));
                outputs.extend(stage_outputs.clone());
                outputs.extend(materialize_rag_outputs(
                    &stage.id,
                    &working_dir,
                    &stage_rag,
                )?);
                outputs_by_stage.insert(stage.id.clone(), stage_outputs);
                break;
            }
        }

        stage_outputs.extend(materialize_stage_outputs(
            stage,
            requirement,
            &working_dir,
            &stage_inputs,
            &stage_rag,
        )?);
        if danger && stage.id == "implementation" {
            fs::write(working_dir.join("danger.txt"), "unsafe-change")?;
        }

        outputs.extend(stage_outputs.clone());
        outputs.extend(materialize_rag_outputs(
            &stage.id,
            &working_dir,
            &stage_rag,
        )?);
        outputs_by_stage.insert(stage.id.clone(), stage_outputs);

        if stage.human_gate && !auto_approve_human_gate {
            pipeline_status = "paused".to_string();
            paused_stage = Some(stage.id.clone());
            error = Some(format!("human gate required at stage {}", stage.id));
            break;
        }

        if autonomy_enabled && stage.id == "testing" {
            let decision = evaluate_decision(&mut forced, run_state.round);
            run_state.risk_score = decision.risk_score;
            run_state.decision_history.push(decision.clone());

            match decision.decision.as_str() {
                "rework" => {
                    run_state.failure_pattern.push(decision.reason);
                    if run_state.round >= run_state.max_rounds {
                        pipeline_status = "failed".to_string();
                        error = Some("max autonomy rounds exceeded".to_string());
                        break;
                    }
                    clear_autonomy_stage_outputs(&workflow.stages, &mut outputs_by_stage);
                    stage_index = implementation_index;
                    continue;
                }
                "rollback" => {
                    if let Some(last) = run_state.last_stable_commit.clone() {
                        rollback_to_checkpoint(&working_dir, &last)?;
                    }
                    pipeline_status = "failed".to_string();
                    error = Some(decision.reason);
                    break;
                }
                "escalate" => {
                    pipeline_status = "paused".to_string();
                    paused_stage = Some(stage.id.clone());
                    error = Some(decision.reason);
                    break;
                }
                _ => {
                    let stable =
                        create_checkpoint(&working_dir, "orchestrator-rs: stable checkpoint")
                            .context("failed to create stable checkpoint")?;
                    run_state.last_stable_commit = Some(stable.commit);
                }
            }
        }

        stage_index += 1;
    }

    if !autonomy_enabled && pipeline_status == "completed" {
        let stable = create_checkpoint(&working_dir, "orchestrator-rs: stable checkpoint")
            .context("failed to create stable checkpoint")?;
        run_state.last_stable_commit = Some(stable.commit);
    }

    Ok(PipelineResult {
        pipeline_id,
        pipeline_status,
        outputs,
        discussions,
        last_stable_commit: run_state.last_stable_commit.clone(),
        run_state,
        working_dir: working_dir.to_string_lossy().to_string(),
        error,
        paused_stage,
    })
}

fn materialize_stage_outputs(
    stage: &StageDefinition,
    requirement: &str,
    working_dir: &Path,
    input_outputs: &[ArtifactOutput],
    rag_snippets: &[RagSnippet],
) -> Result<Vec<ArtifactOutput>> {
    let output_specs = if stage.outputs.is_empty() {
        vec![StageOutputSpec {
            kind: "stage-output".to_string(),
            name: format!("{}.md", stage.id),
        }]
    } else {
        stage.outputs.clone()
    };

    let mut outputs = Vec::new();
    for out in output_specs {
        outputs.extend(materialize_stage_output(
            requirement,
            &stage.id,
            &out.kind,
            &out.name,
            working_dir,
            input_outputs,
            rag_snippets,
        )?);
    }
    Ok(outputs)
}

fn run_stage_discussion(
    pipeline_id: &str,
    stage: &StageDefinition,
    requirement: &str,
    working_dir: &Path,
    inputs: &[ArtifactOutput],
) -> Result<DiscussionRun> {
    let mut participants = vec![stage.agent.clone()];
    for collaborator in &stage.collaborators {
        if !participants.iter().any(|existing| existing == collaborator) {
            participants.push(collaborator.clone());
        }
    }

    let mut bus = MessageBus::default();
    let max_rounds = 4;
    let mut conversation = bus.create_conversation(
        pipeline_id,
        &stage.id,
        &stage.name,
        participants,
        max_rounds,
    );
    let engine = ConvergenceEngine;
    let force_expire = requirement.contains("[[discuss:exhaust]]");
    let force_escalate = requirement.contains("[[discuss:escalate]]");

    for round in 1..=max_rounds {
        bus.send(
            &conversation.id,
            round,
            &stage.agent,
            "discuss",
            &format!("Round {} lead proposal for {}", round, stage.id),
        );
        for collaborator in &stage.collaborators {
            bus.send(
                &conversation.id,
                round,
                collaborator,
                "feedback",
                &format!("Round {} feedback from {}", round, collaborator),
            );
        }

        let messages = bus.get_messages(&conversation.id);
        let convergence = engine.check(
            &conversation,
            &messages,
            round,
            force_expire,
            force_escalate,
        );
        if convergence.should_escalate {
            bus.update_status(&conversation.id, ConversationStatus::Escalated, round);
            break;
        }
        if convergence.converged {
            bus.update_status(&conversation.id, ConversationStatus::Converged, round);
            break;
        }
        if convergence.expired {
            bus.update_status(&conversation.id, ConversationStatus::Expired, round);
            break;
        }
    }

    if let Some(updated) = bus.get_conversation(&conversation.id) {
        conversation = updated;
    }
    if conversation.status == ConversationStatus::Active {
        bus.update_status(&conversation.id, ConversationStatus::Expired, max_rounds);
        if let Some(updated) = bus.get_conversation(&conversation.id) {
            conversation = updated;
        }
    }
    let messages = bus.get_messages(&conversation.id);

    let outputs = vec![
        materialize_discussion_summary_artifact(
            stage,
            requirement,
            working_dir,
            inputs,
            &conversation,
        )?,
        materialize_discussion_transcript(stage, working_dir, &conversation, &messages)?,
    ];
    let record = DiscussionRecord {
        conversation_id: conversation.id.clone(),
        stage_id: stage.id.clone(),
        topic: conversation.topic.clone(),
        status: conversation_status(&conversation.status).to_string(),
        round_count: conversation.round_count,
        max_rounds: conversation.max_rounds,
        participants: conversation.participants.clone(),
        created_at: conversation.created_at.clone(),
        messages: messages
            .iter()
            .map(|m| DiscussionMessageRecord {
                id: m.id.clone(),
                round: m.round,
                from_role: m.from_role.clone(),
                message_type: m.message_type.clone(),
                body: m.body.clone(),
                timestamp: m.timestamp.clone(),
            })
            .collect(),
    };

    Ok(DiscussionRun {
        outputs,
        status: conversation.status,
        record,
    })
}

fn materialize_discussion_summary_artifact(
    stage: &StageDefinition,
    requirement: &str,
    working_dir: &Path,
    inputs: &[ArtifactOutput],
    conversation: &crate::messaging::Conversation,
) -> Result<ArtifactOutput> {
    let artifact_dir = working_dir.join("artifacts").join(&stage.id);
    fs::create_dir_all(&artifact_dir)?;
    let file = artifact_dir.join(format!("discussion-{}.md", stage.id));
    let guard = ExecutionGuard::default();
    let decision = guard.validate_filesystem("writeFile", &file.to_string_lossy());
    if !decision.allowed {
        anyhow::bail!(
            "guardrail violation while writing discussion summary: {}",
            decision.reason.unwrap_or_else(|| "blocked".to_string())
        );
    }

    let mut lines = vec![
        format!("# Discussion: {}", stage.name),
        String::new(),
        format!("- lead: {}", stage.agent),
        format!("- collaborators: {}", stage.collaborators.join(", ")),
        format!("- status: {}", conversation_status(&conversation.status)),
        format!(
            "- rounds: {}/{}",
            conversation.round_count, conversation.max_rounds
        ),
        format!("- requirement: {}", requirement),
    ];
    if !inputs.is_empty() {
        lines.push("- referenced inputs:".to_string());
        for input in inputs {
            lines.push(format!("  - [{}] {}", input.kind, input.name));
        }
    }
    lines.push(String::new());
    fs::write(&file, lines.join("\n"))?;

    Ok(ArtifactOutput {
        stage: stage.id.clone(),
        kind: "discussion-summary".to_string(),
        name: format!("discussion-{}.md", stage.id),
        file_path: stringify_path(file),
    })
}

fn materialize_discussion_transcript(
    stage: &StageDefinition,
    working_dir: &Path,
    conversation: &crate::messaging::Conversation,
    messages: &[crate::messaging::Message],
) -> Result<ArtifactOutput> {
    let artifact_dir = working_dir.join("artifacts").join(&stage.id);
    fs::create_dir_all(&artifact_dir)?;
    let file = artifact_dir.join(format!("discussion-{}-transcript.md", stage.id));
    let guard = ExecutionGuard::default();
    let decision = guard.validate_filesystem("writeFile", &file.to_string_lossy());
    if !decision.allowed {
        anyhow::bail!(
            "guardrail violation while writing discussion transcript: {}",
            decision.reason.unwrap_or_else(|| "blocked".to_string())
        );
    }

    let mut lines = vec![
        format!("conversation_id: {}", conversation.id),
        format!("stage_id: {}", stage.id),
        format!("status: {}", conversation_status(&conversation.status)),
        format!(
            "rounds: {}/{}",
            conversation.round_count, conversation.max_rounds
        ),
        format!("participants: {}", conversation.participants.join(", ")),
        String::new(),
        "---".to_string(),
    ];
    for message in messages {
        lines.push(format!(
            "- [round {}] {} ({}) {}",
            message.round, message.from_role, message.message_type, message.body
        ));
    }
    lines.push(String::new());
    fs::write(&file, lines.join("\n"))?;

    Ok(ArtifactOutput {
        stage: stage.id.clone(),
        kind: "discussion-transcript".to_string(),
        name: format!("discussion-{}-transcript.md", stage.id),
        file_path: stringify_path(file),
    })
}

fn conversation_status(status: &ConversationStatus) -> &'static str {
    match status {
        ConversationStatus::Active => "active",
        ConversationStatus::Converged => "converged",
        ConversationStatus::Expired => "expired",
        ConversationStatus::Escalated => "escalated",
    }
}

fn collect_stage_inputs(
    stage: &StageDefinition,
    outputs_by_stage: &HashMap<String, Vec<ArtifactOutput>>,
) -> Vec<ArtifactOutput> {
    let mut result = Vec::new();
    for input in &stage.inputs {
        if let Some(previous) = outputs_by_stage.get(&input.from_stage) {
            result.extend(previous.iter().filter(|o| o.kind == input.kind).cloned());
        }
    }
    result
}

fn build_stage_index_map(stages: &[StageDefinition]) -> HashMap<String, usize> {
    let mut map = HashMap::new();
    for (idx, stage) in stages.iter().enumerate() {
        map.insert(stage.id.clone(), idx);
    }
    map
}

fn group_outputs_by_stage(outputs: &[ArtifactOutput]) -> HashMap<String, Vec<ArtifactOutput>> {
    let mut grouped = HashMap::new();
    for output in outputs {
        grouped
            .entry(output.stage.clone())
            .or_insert_with(Vec::new)
            .push(output.clone());
    }
    grouped
}

fn trim_outputs_for_resume(
    outputs: Vec<ArtifactOutput>,
    stage_index_map: &HashMap<String, usize>,
    paused_stage: Option<&str>,
    previous_error: Option<&str>,
) -> Vec<ArtifactOutput> {
    let Some(stage_id) = paused_stage else {
        return outputs;
    };
    let Some(paused_idx) = stage_index_map.get(stage_id).copied() else {
        return outputs;
    };
    let rerun_paused = previous_error
        .map(|v| v.contains("discussion escalated"))
        .unwrap_or(false);
    let keep_limit = if rerun_paused {
        paused_idx.saturating_sub(1)
    } else {
        paused_idx
    };
    outputs
        .into_iter()
        .filter(|out| {
            stage_index_map
                .get(&out.stage)
                .map(|idx| *idx <= keep_limit)
                .unwrap_or(false)
        })
        .collect()
}

fn resume_start_index(
    stages: &[StageDefinition],
    paused_stage: Option<&str>,
    previous_error: Option<&str>,
) -> usize {
    let Some(stage_id) = paused_stage else {
        return 0;
    };
    let Some(paused_idx) = stages.iter().position(|s| s.id == stage_id) else {
        return 0;
    };
    let rerun_paused = previous_error
        .map(|v| v.contains("discussion escalated"))
        .unwrap_or(false);
    if rerun_paused {
        paused_idx
    } else {
        paused_idx.saturating_add(1)
    }
}

fn clear_autonomy_stage_outputs(
    stages: &[StageDefinition],
    outputs_by_stage: &mut HashMap<String, Vec<ArtifactOutput>>,
) {
    for stage in stages {
        if matches!(stage.id.as_str(), "implementation" | "review" | "testing") {
            outputs_by_stage.remove(&stage.id);
        }
    }
}

fn materialize_stage_output(
    requirement: &str,
    stage: &str,
    kind: &str,
    name: &str,
    working_dir: &Path,
    input_outputs: &[ArtifactOutput],
    rag_snippets: &[RagSnippet],
) -> Result<Vec<ArtifactOutput>> {
    let artifact_dir = working_dir.join("artifacts").join(stage);
    fs::create_dir_all(&artifact_dir)?;

    let output_file = artifact_dir.join(name);
    let guard = ExecutionGuard::default();
    let decision = guard.validate_filesystem("writeFile", &output_file.to_string_lossy());
    if !decision.allowed {
        anyhow::bail!(
            "guardrail violation while writing output: {}",
            decision.reason.unwrap_or_else(|| "blocked".to_string())
        );
    }
    let mut lines = vec![
        format!("# {}", stage.to_uppercase()),
        String::new(),
        format!("- requirement: {}", requirement),
    ];
    if !input_outputs.is_empty() {
        lines.push("- inputs:".to_string());
        for input in input_outputs {
            lines.push(format!("  - [{}] {}", input.kind, input.name));
        }
    }
    if !rag_snippets.is_empty() {
        lines.push("- rag-hints:".to_string());
        for snippet in rag_snippets.iter().take(3) {
            lines.push(format!(
                "  - ({:.3}) {}:{}",
                snippet.score, snippet.kind, snippet.name
            ));
        }
    }
    lines.push(String::new());
    let content = lines.join("\n");
    fs::write(&output_file, content)?;

    Ok(vec![ArtifactOutput {
        stage: stage.to_string(),
        kind: kind.to_string(),
        name: name.to_string(),
        file_path: stringify_path(output_file),
    }])
}

fn stringify_path(path: PathBuf) -> String {
    path.to_string_lossy().to_string()
}

fn build_rag_snippets(
    stage: &str,
    requirement: &str,
    working_dir: &Path,
    input_outputs: &[ArtifactOutput],
    failure_pattern: &[String],
) -> Result<Vec<RagSnippet>> {
    let mut docs = load_input_docs(input_outputs)?;
    docs.extend(collect_workspace_docs(working_dir, 40, 20_000)?);
    for (idx, failure) in failure_pattern.iter().enumerate() {
        docs.push(RagDocument {
            kind: "failure-log".to_string(),
            name: format!("failure-{}", idx + 1),
            content: failure.clone(),
        });
    }
    if docs.is_empty() {
        return Ok(Vec::new());
    }

    let mut query_lines = vec![requirement.to_string(), stage.to_string()];
    query_lines.extend(failure_pattern.iter().rev().take(2).cloned());
    let retriever = RagRetriever::default();
    Ok(retriever.retrieve(&query_lines.join("\n"), &docs))
}

fn load_input_docs(input_outputs: &[ArtifactOutput]) -> Result<Vec<RagDocument>> {
    let mut docs = Vec::new();
    for input in input_outputs {
        let content = fs::read_to_string(&input.file_path).unwrap_or_else(|_| String::new());
        docs.push(RagDocument {
            kind: input.kind.clone(),
            name: input.name.clone(),
            content,
        });
    }
    Ok(docs)
}

fn collect_workspace_docs(
    root_dir: &Path,
    max_files: usize,
    max_file_size: u64,
) -> Result<Vec<RagDocument>> {
    let mut docs = Vec::new();
    walk_workspace_docs(root_dir, root_dir, 0, max_files, max_file_size, &mut docs)?;
    Ok(docs)
}

fn walk_workspace_docs(
    root_dir: &Path,
    current_dir: &Path,
    depth: usize,
    max_files: usize,
    max_file_size: u64,
    docs: &mut Vec<RagDocument>,
) -> Result<()> {
    if depth > 3 || docs.len() >= max_files {
        return Ok(());
    }
    let entries = match fs::read_dir(current_dir) {
        Ok(entries) => entries,
        Err(_) => return Ok(()),
    };
    for entry in entries {
        if docs.len() >= max_files {
            break;
        }
        let entry = match entry {
            Ok(v) => v,
            Err(_) => continue,
        };
        let path = entry.path();
        let file_name = path
            .file_name()
            .and_then(|s| s.to_str())
            .unwrap_or_default()
            .to_string();

        if path.is_dir() {
            if matches!(
                file_name.as_str(),
                ".git" | "node_modules" | "target" | "dist" | "coverage"
            ) {
                continue;
            }
            let _ = walk_workspace_docs(root_dir, &path, depth + 1, max_files, max_file_size, docs);
            continue;
        }
        if !path.is_file() {
            continue;
        }
        if !is_allowed_ext(&path) {
            continue;
        }
        let meta = match fs::metadata(&path) {
            Ok(v) => v,
            Err(_) => continue,
        };
        if meta.len() > max_file_size {
            continue;
        }

        let rel = path
            .strip_prefix(root_dir)
            .map(|p| p.to_string_lossy().to_string())
            .unwrap_or_else(|_| path.to_string_lossy().to_string());
        let content = fs::read_to_string(&path).unwrap_or_else(|_| String::new());
        docs.push(RagDocument {
            kind: "workspace-file".to_string(),
            name: rel,
            content,
        });
    }
    Ok(())
}

fn is_allowed_ext(path: &Path) -> bool {
    let ext = path
        .extension()
        .and_then(|s| s.to_str())
        .unwrap_or_default()
        .to_lowercase();
    matches!(
        ext.as_str(),
        "rs" | "ts" | "tsx" | "js" | "jsx" | "json" | "md" | "yaml" | "yml" | "toml"
    )
}

fn materialize_rag_outputs(
    stage: &str,
    working_dir: &Path,
    snippets: &[RagSnippet],
) -> Result<Vec<ArtifactOutput>> {
    if snippets.is_empty() {
        return Ok(Vec::new());
    }
    let artifact_dir = working_dir.join("artifacts").join(stage);
    fs::create_dir_all(&artifact_dir)?;
    let guard = ExecutionGuard::default();
    let mut outputs = Vec::new();
    for (idx, snippet) in snippets.iter().take(3).enumerate() {
        let safe_name = snippet
            .name
            .chars()
            .map(|c| if c.is_ascii_alphanumeric() { c } else { '-' })
            .collect::<String>();
        let name = format!("rag-{}-{}.md", idx + 1, safe_name);
        let file = artifact_dir.join(&name);
        let decision = guard.validate_filesystem("writeFile", &file.to_string_lossy());
        if !decision.allowed {
            continue;
        }
        let body = format!(
            "source_kind: {}\nsource_name: {}\nscore: {:.3}\n\n{}",
            snippet.kind, snippet.name, snippet.score, snippet.content
        );
        fs::write(&file, body)?;
        outputs.push(ArtifactOutput {
            stage: stage.to_string(),
            kind: "rag-context".to_string(),
            name,
            file_path: stringify_path(file),
        });
    }
    Ok(outputs)
}

fn evaluate_decision(forced: &mut Vec<String>, round: u32) -> DecisionRecord {
    let decision = if forced.is_empty() {
        "continue".to_string()
    } else {
        forced.remove(0)
    };

    let (reason, risk_score) = match decision.as_str() {
        "rework" => ("needs another pass".to_string(), 0.65),
        "rollback" => ("high risk detected".to_string(), 0.95),
        "escalate" => ("requires human intervention".to_string(), 0.80),
        _ => ("looks safe".to_string(), 0.20),
    };

    DecisionRecord {
        round,
        decision,
        reason,
        risk_score,
        timestamp: Utc::now().to_rfc3339(),
    }
}

fn parse_forced_decisions(requirement: &str) -> Vec<String> {
    let re = Regex::new(r"\[\[decisions:([a-z,\s]+)\]\]").expect("valid regex");
    let Some(caps) = re.captures(requirement) else {
        return Vec::new();
    };
    let Some(raw) = caps.get(1) else {
        return Vec::new();
    };
    raw.as_str()
        .split(',')
        .map(|v| v.trim().to_lowercase())
        .filter(|v| matches!(v.as_str(), "continue" | "rework" | "rollback" | "escalate"))
        .collect()
}

fn has_danger_marker(requirement: &str) -> bool {
    requirement.contains("[[danger:true]]")
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;
    use std::fs;
    use std::path::Path;

    use super::{
        resume_workflow_with_catalog, run_workflow, run_workflow_with_catalog, ResumeContext,
    };
    use crate::workflow_config::{
        StageDefinition, StageOutputSpec, WorkflowCatalog, WorkflowDefinition,
    };

    #[test]
    fn workflow_generates_artifact_outputs() {
        let temp = tempfile::tempdir().expect("tempdir");
        let result = run_workflow("build me a plan", "mvp", temp.path()).expect("run workflow");
        assert_eq!(result.pipeline_status, "completed");
        assert!(!result.pipeline_id.is_empty());
        let prd = result
            .outputs
            .iter()
            .find(|o| o.kind == "prd")
            .expect("prd output");
        assert_eq!(prd.name, "PRD.md");
        assert!(std::path::Path::new(&prd.file_path).exists());
        assert_eq!(result.run_state.round, 0);
    }

    #[test]
    fn autonomy_rework_then_continue_tracks_rounds_and_decisions() {
        let temp = tempfile::tempdir().expect("tempdir");
        let result = run_workflow(
            "[[decisions:rework,continue]] build feature",
            "autonomy",
            temp.path(),
        )
        .expect("run autonomy");

        assert_eq!(result.pipeline_status, "completed");
        assert_eq!(result.run_state.round, 2);
        assert_eq!(result.run_state.decision_history.len(), 2);
        assert_eq!(result.run_state.decision_history[0].decision, "rework");
        assert_eq!(result.run_state.decision_history[1].decision, "continue");
    }

    #[test]
    fn autonomy_rollback_restores_workspace() {
        let temp = tempfile::tempdir().expect("tempdir");
        let result = run_workflow(
            "[[decisions:rollback]] [[danger:true]] risky change",
            "autonomy",
            temp.path(),
        )
        .expect("run rollback");

        assert_eq!(result.pipeline_status, "failed");
        let danger = Path::new(&result.working_dir).join("danger.txt");
        assert!(!danger.exists());
        let all = fs::read_dir(Path::new(&result.working_dir))
            .expect("read dir")
            .collect::<Vec<_>>();
        assert!(!all.is_empty());
    }

    #[test]
    fn autonomy_generates_rag_context_outputs() {
        let temp = tempfile::tempdir().expect("tempdir");
        let result = run_workflow(
            "[[decisions:continue]] build with context reuse",
            "autonomy",
            temp.path(),
        )
        .expect("run autonomy");
        let rag = result
            .outputs
            .iter()
            .filter(|o| o.kind == "rag-context")
            .collect::<Vec<_>>();
        assert!(!rag.is_empty());
    }

    #[test]
    fn default_workflow_emits_discussion_summary_for_collaborator_stage() {
        let temp = tempfile::tempdir().expect("tempdir");
        let result = run_workflow("build multi-agent flow", "default", temp.path())
            .expect("run default workflow");
        let has_discussion = result
            .outputs
            .iter()
            .any(|o| o.kind == "discussion-summary");
        assert!(has_discussion);
    }

    #[test]
    fn default_workflow_emits_discussion_transcript_for_collaborator_stage() {
        let temp = tempfile::tempdir().expect("tempdir");
        let result = run_workflow("build multi-agent flow", "default", temp.path())
            .expect("run default workflow");
        let transcript = result
            .outputs
            .iter()
            .find(|o| o.kind == "discussion-transcript")
            .expect("discussion transcript");
        assert!(std::path::Path::new(&transcript.file_path).exists());
    }

    #[test]
    fn discussion_exhaustion_marks_transcript_expired_but_pipeline_continues() {
        let temp = tempfile::tempdir().expect("tempdir");
        let result = run_workflow(
            "[[discuss:exhaust]] build multi-agent flow",
            "default",
            temp.path(),
        )
        .expect("run default workflow");
        assert_eq!(result.pipeline_status, "completed");
        let transcript = result
            .outputs
            .iter()
            .find(|o| o.kind == "discussion-transcript")
            .expect("discussion transcript");
        let content = fs::read_to_string(&transcript.file_path).expect("read transcript");
        assert!(content.contains("status: expired"));
    }

    #[test]
    fn discussion_escalation_pauses_pipeline_at_discussion_stage() {
        let temp = tempfile::tempdir().expect("tempdir");
        let result = run_workflow(
            "[[discuss:escalate]] build multi-agent flow",
            "default",
            temp.path(),
        )
        .expect("run default workflow");
        assert_eq!(result.pipeline_status, "paused");
        assert_eq!(result.paused_stage.as_deref(), Some("prd"));
        assert!(result
            .error
            .as_deref()
            .unwrap_or_default()
            .contains("discussion escalated"));
    }

    #[test]
    fn human_gate_stage_pauses_pipeline() {
        let temp = tempfile::tempdir().expect("tempdir");
        let mut workflows = HashMap::new();
        workflows.insert(
            "custom".to_string(),
            WorkflowDefinition {
                name: "Custom".to_string(),
                stages: vec![StageDefinition {
                    id: "plan".to_string(),
                    name: "Plan".to_string(),
                    agent: "pm".to_string(),
                    collaborators: Vec::new(),
                    human_gate: true,
                    inputs: Vec::new(),
                    outputs: vec![StageOutputSpec {
                        kind: "plan-doc".to_string(),
                        name: "plan.md".to_string(),
                    }],
                }],
            },
        );
        let catalog = WorkflowCatalog { workflows };
        let result = run_workflow_with_catalog("need review", "custom", temp.path(), &catalog)
            .expect("run workflow");
        assert_eq!(result.pipeline_status, "paused");
        assert_eq!(result.paused_stage.as_deref(), Some("plan"));
    }

    #[test]
    fn resume_continues_after_human_gate_stage() {
        let temp = tempfile::tempdir().expect("tempdir");
        let mut workflows = HashMap::new();
        workflows.insert(
            "custom".to_string(),
            WorkflowDefinition {
                name: "Custom".to_string(),
                stages: vec![
                    StageDefinition {
                        id: "plan".to_string(),
                        name: "Plan".to_string(),
                        agent: "pm".to_string(),
                        collaborators: Vec::new(),
                        human_gate: true,
                        inputs: Vec::new(),
                        outputs: vec![StageOutputSpec {
                            kind: "plan-doc".to_string(),
                            name: "plan.md".to_string(),
                        }],
                    },
                    StageDefinition {
                        id: "build".to_string(),
                        name: "Build".to_string(),
                        agent: "coder".to_string(),
                        collaborators: Vec::new(),
                        human_gate: false,
                        inputs: vec![crate::workflow_config::StageInput {
                            from_stage: "plan".to_string(),
                            kind: "plan-doc".to_string(),
                        }],
                        outputs: vec![StageOutputSpec {
                            kind: "delivery".to_string(),
                            name: "build.md".to_string(),
                        }],
                    },
                ],
            },
        );
        let catalog = WorkflowCatalog { workflows };
        let paused =
            run_workflow_with_catalog("need review", "custom", temp.path(), &catalog).expect("run");
        assert_eq!(paused.pipeline_status, "paused");

        let resumed = resume_workflow_with_catalog(
            "need review",
            "custom",
            temp.path(),
            &catalog,
            ResumeContext {
                pipeline_id: paused.pipeline_id.clone(),
                paused_stage: paused.paused_stage.clone(),
                previous_outputs: paused.outputs.clone(),
                previous_error: paused.error.clone(),
            },
        )
        .expect("resume");

        assert_eq!(resumed.pipeline_status, "completed");
        assert_eq!(resumed.pipeline_id, paused.pipeline_id);
        assert!(resumed.outputs.iter().any(|o| o.kind == "delivery"));
    }
}
