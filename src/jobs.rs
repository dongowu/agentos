use std::fs;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use chrono::Utc;
use rusqlite::{params, Connection, OptionalExtension};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::pipeline::{
    resume_workflow_with_catalog, run_workflow_with_catalog, ArtifactOutput, DiscussionRecord,
    ResumeContext,
};
use crate::workflow_config::WorkflowCatalog;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobResult {
    pub job_id: String,
    pub status: String,
    pub pipeline_id: Option<String>,
    pub pipeline_status: Option<String>,
    pub outputs: Vec<ArtifactOutput>,
    pub paused_stage: Option<String>,
    pub error: Option<String>,
    pub updated_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HumanDecision {
    pub id: String,
    pub pipeline_id: String,
    pub stage_id: String,
    pub description: String,
    pub status: String,
    pub reason: Option<String>,
    pub created_at: String,
    pub resolved_at: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PipelineTrace {
    pub pipeline_id: String,
    pub job: Option<JobResult>,
    pub decisions: Vec<HumanDecision>,
    pub conversations: Vec<ConversationTrace>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConversationTrace {
    pub id: String,
    pub pipeline_id: String,
    pub stage_id: String,
    pub topic: String,
    pub status: String,
    pub round_count: u32,
    pub max_rounds: u32,
    pub participants: Vec<String>,
    pub created_at: String,
    pub messages: Vec<MessageTrace>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MessageTrace {
    pub id: String,
    pub conversation_id: String,
    pub pipeline_id: String,
    pub round: u32,
    pub from_role: String,
    pub message_type: String,
    pub body: String,
    pub timestamp: String,
}

pub struct JobService {
    db_path: PathBuf,
    data_dir: PathBuf,
    workflows: WorkflowCatalog,
}

impl JobService {
    #[allow(dead_code)]
    pub fn new(db_path: impl AsRef<Path>, data_dir: impl AsRef<Path>) -> Result<Self> {
        Self::new_with_workflow_file(db_path, data_dir, None::<&Path>)
    }

    pub fn new_with_workflow_file(
        db_path: impl AsRef<Path>,
        data_dir: impl AsRef<Path>,
        workflow_file: Option<impl AsRef<Path>>,
    ) -> Result<Self> {
        let db_path = db_path.as_ref().to_path_buf();
        let data_dir = data_dir.as_ref().to_path_buf();
        if let Some(parent) = db_path.parent() {
            fs::create_dir_all(parent)?;
        }
        fs::create_dir_all(&data_dir)?;
        let workflows =
            WorkflowCatalog::load_or_default(workflow_file.as_ref().map(|p| p.as_ref()))?;

        let service = Self {
            db_path,
            data_dir,
            workflows,
        };
        service.init_schema()?;
        Ok(service)
    }

    pub fn submit(&self, requirement: &str, workflow_id: &str) -> Result<String> {
        let job_id = format!("job_{}", Uuid::new_v4().simple());
        let now = Utc::now().to_rfc3339();
        let conn = self.open()?;
        conn.execute(
            "INSERT INTO jobs (id, requirement, workflow_id, status, created_at, updated_at)
             VALUES (?1, ?2, ?3, 'queued', ?4, ?4)",
            params![job_id, requirement, workflow_id, now],
        )?;
        Ok(job_id)
    }

    pub fn run_job(&self, job_id: &str) -> Result<()> {
        let conn = self.open()?;
        let row = Self::fetch_row(&conn, job_id)?.context("job not found")?;
        conn.execute(
            "UPDATE jobs SET status = 'running', error = NULL, updated_at = ?2 WHERE id = ?1",
            params![job_id, Utc::now().to_rfc3339()],
        )?;

        let run = run_workflow_with_catalog(
            &row.requirement,
            &row.workflow_id,
            &self.data_dir,
            &self.workflows,
        );
        match run {
            Ok(result) => {
                if result.pipeline_status == "paused" {
                    let pipeline_id = result.pipeline_id.clone();
                    let reason = result
                        .error
                        .clone()
                        .unwrap_or_else(|| "human review required".to_string());
                    Self::create_pending_decision(
                        &conn,
                        &pipeline_id,
                        result.paused_stage.as_deref().unwrap_or("testing"),
                        &format!("Pipeline paused: {}", reason),
                    )?;
                }
                Self::persist_discussions(&conn, &result.pipeline_id, &result.discussions)?;
                let payload = JobResult {
                    job_id: row.id,
                    status: result.pipeline_status.clone(),
                    pipeline_id: Some(result.pipeline_id),
                    pipeline_status: Some(result.pipeline_status),
                    outputs: result.outputs,
                    paused_stage: result.paused_stage,
                    error: result.error,
                    updated_at: Utc::now().to_rfc3339(),
                };
                conn.execute(
                    "UPDATE jobs
                     SET status = ?2, pipeline_id = ?3, error = ?4, result_json = ?5, updated_at = ?6
                     WHERE id = ?1",
                    params![
                        payload.job_id,
                        payload.status,
                        payload.pipeline_id,
                        payload.error,
                        serde_json::to_string(&payload)?,
                        payload.updated_at
                    ],
                )?;
                Ok(())
            }
            Err(err) => {
                let message = err.to_string();
                let payload = JobResult {
                    job_id: row.id,
                    status: "failed".to_string(),
                    pipeline_id: None,
                    pipeline_status: Some("failed".to_string()),
                    outputs: Vec::new(),
                    paused_stage: None,
                    error: Some(message.clone()),
                    updated_at: Utc::now().to_rfc3339(),
                };
                conn.execute(
                    "UPDATE jobs
                     SET status = 'failed', error = ?2, result_json = ?3, updated_at = ?4
                     WHERE id = ?1",
                    params![
                        payload.job_id,
                        message,
                        serde_json::to_string(&payload)?,
                        payload.updated_at
                    ],
                )?;
                Ok(())
            }
        }
    }

    pub fn resume_job(&self, job_id: &str) -> Result<()> {
        let conn = self.open()?;
        let row = Self::fetch_row(&conn, job_id)?.context("job not found")?;
        if row.status != "paused" {
            anyhow::bail!("job {} is not paused", job_id);
        }
        let previous = Self::into_result(row.clone());
        let pipeline_id = previous
            .pipeline_id
            .clone()
            .context("paused job has no pipeline id")?;
        let pending_count = conn.query_row(
            "SELECT COUNT(1) FROM human_decisions WHERE pipeline_id = ?1 AND status = 'pending'",
            params![pipeline_id],
            |r| r.get::<_, i64>(0),
        )?;
        if pending_count > 0 {
            anyhow::bail!("pipeline has unresolved pending decisions");
        }

        conn.execute(
            "UPDATE jobs SET status = 'running', error = NULL, updated_at = ?2 WHERE id = ?1",
            params![job_id, Utc::now().to_rfc3339()],
        )?;

        let resumed = resume_workflow_with_catalog(
            &row.requirement,
            &row.workflow_id,
            &self.data_dir,
            &self.workflows,
            ResumeContext {
                pipeline_id: pipeline_id.clone(),
                paused_stage: previous.paused_stage.clone(),
                previous_outputs: previous.outputs.clone(),
                previous_error: previous.error.clone(),
            },
        );

        match resumed {
            Ok(result) => {
                if result.pipeline_status == "paused" {
                    let reason = result
                        .error
                        .clone()
                        .unwrap_or_else(|| "human review required".to_string());
                    Self::create_pending_decision(
                        &conn,
                        &pipeline_id,
                        result.paused_stage.as_deref().unwrap_or("testing"),
                        &format!("Pipeline paused: {}", reason),
                    )?;
                }
                Self::persist_discussions(&conn, &pipeline_id, &result.discussions)?;
                let payload = JobResult {
                    job_id: row.id,
                    status: result.pipeline_status.clone(),
                    pipeline_id: Some(result.pipeline_id),
                    pipeline_status: Some(result.pipeline_status),
                    outputs: result.outputs,
                    paused_stage: result.paused_stage,
                    error: result.error,
                    updated_at: Utc::now().to_rfc3339(),
                };
                conn.execute(
                    "UPDATE jobs
                     SET status = ?2, pipeline_id = ?3, error = ?4, result_json = ?5, updated_at = ?6
                     WHERE id = ?1",
                    params![
                        payload.job_id,
                        payload.status,
                        payload.pipeline_id,
                        payload.error,
                        serde_json::to_string(&payload)?,
                        payload.updated_at
                    ],
                )?;
                Ok(())
            }
            Err(err) => {
                let message = err.to_string();
                let payload = JobResult {
                    job_id: row.id,
                    status: "failed".to_string(),
                    pipeline_id: Some(pipeline_id.clone()),
                    pipeline_status: Some("failed".to_string()),
                    outputs: previous.outputs,
                    paused_stage: None,
                    error: Some(message.clone()),
                    updated_at: Utc::now().to_rfc3339(),
                };
                conn.execute(
                    "UPDATE jobs
                     SET status = 'failed', pipeline_id = ?3, error = ?2, result_json = ?4, updated_at = ?5
                     WHERE id = ?1",
                    params![
                        payload.job_id,
                        message,
                        payload.pipeline_id,
                        serde_json::to_string(&payload)?,
                        payload.updated_at
                    ],
                )?;
                Ok(())
            }
        }
    }

    pub fn process_queued(&self, limit: usize) -> Result<usize> {
        let conn = self.open()?;
        let mut stmt = conn.prepare(
            "SELECT id FROM jobs WHERE status = 'queued' ORDER BY created_at ASC LIMIT ?1",
        )?;
        let queued: Vec<String> = stmt
            .query_map(params![limit as i64], |r| r.get::<_, String>(0))?
            .collect::<Result<Vec<_>, _>>()?;

        for id in &queued {
            self.run_job(id)?;
        }
        Ok(queued.len())
    }

    pub fn get_status(&self, job_id: &str) -> Result<Option<JobResult>> {
        let conn = self.open()?;
        let row = Self::fetch_row(&conn, job_id)?;
        Ok(row.map(Self::into_result))
    }

    pub fn get_result(&self, job_id: &str) -> Result<Option<JobResult>> {
        self.get_status(job_id)
    }

    pub fn list_pending_decisions(&self, pipeline_id: Option<&str>) -> Result<Vec<HumanDecision>> {
        let conn = self.open()?;
        let sql = if pipeline_id.is_some() {
            "SELECT id, pipeline_id, stage_id, description, status, reason, created_at, resolved_at
             FROM human_decisions WHERE status = 'pending' AND pipeline_id = ?1 ORDER BY created_at ASC"
        } else {
            "SELECT id, pipeline_id, stage_id, description, status, reason, created_at, resolved_at
             FROM human_decisions WHERE status = 'pending' ORDER BY created_at ASC"
        };

        let mut stmt = conn.prepare(sql)?;
        let rows = if let Some(pid) = pipeline_id {
            stmt.query_map(params![pid], Self::map_human_decision)?
        } else {
            stmt.query_map([], Self::map_human_decision)?
        };

        rows.collect::<Result<Vec<_>, _>>()
            .context("failed to list pending decisions")
    }

    pub fn approve_decision(&self, decision_id: &str) -> Result<()> {
        let conn = self.open()?;
        conn.execute(
            "UPDATE human_decisions
             SET status = 'approved', resolved_at = ?2
             WHERE id = ?1",
            params![decision_id, Utc::now().to_rfc3339()],
        )?;
        Ok(())
    }

    pub fn reject_decision(&self, decision_id: &str, reason: Option<&str>) -> Result<()> {
        let conn = self.open()?;
        conn.execute(
            "UPDATE human_decisions
             SET status = 'rejected', reason = ?2, resolved_at = ?3
             WHERE id = ?1",
            params![decision_id, reason, Utc::now().to_rfc3339()],
        )?;
        Ok(())
    }

    pub fn trace_pipeline(&self, pipeline_id: &str) -> Result<PipelineTrace> {
        let conn = self.open()?;

        let job = conn
            .query_row(
                "SELECT id, requirement, workflow_id, pipeline_id, status, error, result_json, updated_at
                 FROM jobs WHERE pipeline_id = ?1 ORDER BY updated_at DESC LIMIT 1",
                params![pipeline_id],
                |r| {
                    Ok(JobRow {
                        id: r.get(0)?,
                        requirement: r.get(1)?,
                        workflow_id: r.get(2)?,
                        pipeline_id: r.get(3)?,
                        status: r.get(4)?,
                        error: r.get(5)?,
                        result_json: r.get(6)?,
                        updated_at: r.get(7)?,
                    })
                },
            )
            .optional()
            .context("failed to query pipeline job")?
            .map(Self::into_result);

        let mut decision_stmt = conn.prepare(
            "SELECT id, pipeline_id, stage_id, description, status, reason, created_at, resolved_at
             FROM human_decisions WHERE pipeline_id = ?1 ORDER BY created_at ASC",
        )?;
        let decisions = decision_stmt
            .query_map(params![pipeline_id], Self::map_human_decision)?
            .collect::<Result<Vec<_>, _>>()
            .context("failed to fetch pipeline decisions")?;

        let mut conv_stmt = conn.prepare(
            "SELECT id, pipeline_id, stage_id, topic, status, round_count, max_rounds, participants_json, created_at
             FROM conversations WHERE pipeline_id = ?1 ORDER BY created_at ASC",
        )?;
        let rows = conv_stmt
            .query_map(params![pipeline_id], |r| {
                Ok((
                    r.get::<_, String>(0)?,
                    r.get::<_, String>(1)?,
                    r.get::<_, String>(2)?,
                    r.get::<_, String>(3)?,
                    r.get::<_, String>(4)?,
                    r.get::<_, i64>(5)?,
                    r.get::<_, i64>(6)?,
                    r.get::<_, String>(7)?,
                    r.get::<_, String>(8)?,
                ))
            })?
            .collect::<Result<Vec<_>, _>>()
            .context("failed to fetch conversations")?;

        let mut conversations = Vec::new();
        for (
            id,
            pid,
            stage_id,
            topic,
            status,
            round_count,
            max_rounds,
            participants_json,
            created_at,
        ) in rows
        {
            let participants =
                serde_json::from_str::<Vec<String>>(&participants_json).unwrap_or_default();
            let messages = Self::fetch_messages_for_conversation(&conn, pipeline_id, &id)?;
            conversations.push(ConversationTrace {
                id,
                pipeline_id: pid,
                stage_id,
                topic,
                status,
                round_count: round_count as u32,
                max_rounds: max_rounds as u32,
                participants,
                created_at,
                messages,
            });
        }

        Ok(PipelineTrace {
            pipeline_id: pipeline_id.to_string(),
            job,
            decisions,
            conversations,
        })
    }

    fn into_result(row: JobRow) -> JobResult {
        if let Some(snapshot) = row.result_json {
            if let Ok(parsed) = serde_json::from_str::<JobResult>(&snapshot) {
                return parsed;
            }
        }
        JobResult {
            job_id: row.id,
            status: row.status,
            pipeline_id: row.pipeline_id,
            pipeline_status: None,
            outputs: Vec::new(),
            paused_stage: None,
            error: row.error,
            updated_at: row.updated_at,
        }
    }

    fn fetch_row(conn: &Connection, job_id: &str) -> Result<Option<JobRow>> {
        conn.query_row(
            "SELECT id, requirement, workflow_id, pipeline_id, status, error, result_json, updated_at
             FROM jobs WHERE id = ?1",
            params![job_id],
            |r| {
                Ok(JobRow {
                    id: r.get(0)?,
                    requirement: r.get(1)?,
                    workflow_id: r.get(2)?,
                    pipeline_id: r.get(3)?,
                    status: r.get(4)?,
                    error: r.get(5)?,
                    result_json: r.get(6)?,
                    updated_at: r.get(7)?,
                })
            },
        )
        .optional()
        .context("failed to fetch job row")
    }

    fn open(&self) -> Result<Connection> {
        Connection::open(&self.db_path).context("failed to open sqlite")
    }

    fn create_pending_decision(
        conn: &Connection,
        pipeline_id: &str,
        stage_id: &str,
        description: &str,
    ) -> Result<()> {
        let id = format!("dec_{}", Uuid::new_v4().simple());
        let now = Utc::now().to_rfc3339();
        conn.execute(
            "INSERT INTO human_decisions
             (id, pipeline_id, stage_id, description, status, created_at)
             VALUES (?1, ?2, ?3, ?4, 'pending', ?5)",
            params![id, pipeline_id, stage_id, description, now],
        )?;
        Ok(())
    }

    fn map_human_decision(r: &rusqlite::Row<'_>) -> rusqlite::Result<HumanDecision> {
        Ok(HumanDecision {
            id: r.get(0)?,
            pipeline_id: r.get(1)?,
            stage_id: r.get(2)?,
            description: r.get(3)?,
            status: r.get(4)?,
            reason: r.get(5)?,
            created_at: r.get(6)?,
            resolved_at: r.get(7)?,
        })
    }

    fn persist_discussions(
        conn: &Connection,
        pipeline_id: &str,
        discussions: &[DiscussionRecord],
    ) -> Result<()> {
        conn.execute(
            "DELETE FROM messages WHERE pipeline_id = ?1",
            params![pipeline_id],
        )?;
        conn.execute(
            "DELETE FROM conversations WHERE pipeline_id = ?1",
            params![pipeline_id],
        )?;

        for discussion in discussions {
            conn.execute(
                "INSERT INTO conversations
                 (id, pipeline_id, stage_id, topic, status, round_count, max_rounds, participants_json, created_at)
                 VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)",
                params![
                    discussion.conversation_id,
                    pipeline_id,
                    discussion.stage_id,
                    discussion.topic,
                    discussion.status,
                    discussion.round_count as i64,
                    discussion.max_rounds as i64,
                    serde_json::to_string(&discussion.participants)?,
                    discussion.created_at
                ],
            )?;

            for message in &discussion.messages {
                conn.execute(
                    "INSERT INTO messages
                     (id, conversation_id, pipeline_id, round, from_role, message_type, body, timestamp)
                     VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)",
                    params![
                        message.id,
                        discussion.conversation_id,
                        pipeline_id,
                        message.round as i64,
                        message.from_role,
                        message.message_type,
                        message.body,
                        message.timestamp
                    ],
                )?;
            }
        }
        Ok(())
    }

    fn fetch_messages_for_conversation(
        conn: &Connection,
        pipeline_id: &str,
        conversation_id: &str,
    ) -> Result<Vec<MessageTrace>> {
        let mut stmt = conn.prepare(
            "SELECT id, conversation_id, pipeline_id, round, from_role, message_type, body, timestamp
             FROM messages
             WHERE pipeline_id = ?1 AND conversation_id = ?2
             ORDER BY round ASC, timestamp ASC",
        )?;
        let rows = stmt.query_map(params![pipeline_id, conversation_id], |r| {
            Ok(MessageTrace {
                id: r.get(0)?,
                conversation_id: r.get(1)?,
                pipeline_id: r.get(2)?,
                round: r.get::<_, i64>(3)? as u32,
                from_role: r.get(4)?,
                message_type: r.get(5)?,
                body: r.get(6)?,
                timestamp: r.get(7)?,
            })
        })?;
        rows.collect::<Result<Vec<_>, _>>()
            .context("failed to fetch conversation messages")
    }

    fn init_schema(&self) -> Result<()> {
        let conn = self.open()?;
        conn.execute_batch(
            "
            CREATE TABLE IF NOT EXISTS jobs (
              id TEXT PRIMARY KEY,
              requirement TEXT NOT NULL,
              workflow_id TEXT NOT NULL,
              pipeline_id TEXT,
              status TEXT NOT NULL DEFAULT 'queued',
              error TEXT,
              result_json TEXT,
              created_at TEXT NOT NULL,
              updated_at TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS human_decisions (
              id TEXT PRIMARY KEY,
              pipeline_id TEXT NOT NULL,
              stage_id TEXT NOT NULL,
              description TEXT NOT NULL,
              status TEXT NOT NULL DEFAULT 'pending',
              reason TEXT,
              created_at TEXT NOT NULL,
              resolved_at TEXT
            );

            CREATE TABLE IF NOT EXISTS conversations (
              id TEXT PRIMARY KEY,
              pipeline_id TEXT NOT NULL,
              stage_id TEXT NOT NULL,
              topic TEXT NOT NULL,
              status TEXT NOT NULL,
              round_count INTEGER NOT NULL,
              max_rounds INTEGER NOT NULL,
              participants_json TEXT NOT NULL,
              created_at TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS messages (
              id TEXT PRIMARY KEY,
              conversation_id TEXT NOT NULL,
              pipeline_id TEXT NOT NULL,
              round INTEGER NOT NULL,
              from_role TEXT NOT NULL,
              message_type TEXT NOT NULL,
              body TEXT NOT NULL,
              timestamp TEXT NOT NULL
            );
            ",
        )
        .context("failed to initialize jobs schema")
    }
}

#[derive(Debug, Clone)]
struct JobRow {
    id: String,
    requirement: String,
    workflow_id: String,
    pipeline_id: Option<String>,
    status: String,
    error: Option<String>,
    result_json: Option<String>,
    updated_at: String,
}

#[cfg(test)]
mod tests {
    use std::fs;

    use super::JobService;

    #[test]
    fn submit_and_process_job_to_completion() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let service = JobService::new(&db_path, &data_dir).expect("service");

        let job_id = service.submit("build prd", "mvp").expect("submit");
        let initial = service
            .get_status(&job_id)
            .expect("get status")
            .expect("status row");
        assert_eq!(initial.status, "queued");

        let processed = service.process_queued(10).expect("process");
        assert_eq!(processed, 1);

        let done = service
            .get_result(&job_id)
            .expect("get result")
            .expect("result row");
        assert_eq!(done.status, "completed");
        assert!(done.pipeline_id.is_some());
        assert!(!done.outputs.is_empty());
    }

    #[test]
    fn escalate_decision_creates_pending_human_gate() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let service = JobService::new(&db_path, &data_dir).expect("service");

        let job_id = service
            .submit("[[decisions:escalate]] review risky release", "autonomy")
            .expect("submit");
        service.process_queued(10).expect("process");

        let status = service.get_status(&job_id).expect("status").expect("row");
        assert_eq!(status.status, "paused");

        let pending = service.list_pending_decisions(None).expect("pending");
        assert_eq!(pending.len(), 1);
        assert_eq!(pending[0].status, "pending");

        service
            .approve_decision(&pending[0].id)
            .expect("approve decision");
        let refreshed = service
            .list_pending_decisions(None)
            .expect("pending refreshed");
        assert!(refreshed.is_empty());
    }

    #[test]
    fn discussion_escalation_creates_pending_human_gate() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let service = JobService::new(&db_path, &data_dir).expect("service");

        let job_id = service
            .submit("[[discuss:escalate]] build multi-agent flow", "default")
            .expect("submit");
        service.process_queued(10).expect("process");

        let status = service.get_status(&job_id).expect("status").expect("row");
        assert_eq!(status.status, "paused");

        let pending = service.list_pending_decisions(None).expect("pending");
        assert_eq!(pending.len(), 1);
        assert_eq!(pending[0].status, "pending");
        assert_eq!(pending[0].stage_id, "prd");
    }

    #[test]
    fn approved_human_gate_job_can_resume_to_completion() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let workflow_file = temp.path().join("workflows.yaml");
        fs::write(
            &workflow_file,
            r#"
workflows:
  resume_flow:
    name: Resume Flow
    stages:
      - id: plan
        name: Plan
        agent: pm
        humanGate: true
        outputs:
          - type: plan-doc
            name: plan.md
      - id: build
        name: Build
        agent: coder
        inputs:
          - fromStage: plan
            type: plan-doc
        outputs:
          - type: delivery
            name: build.md
"#,
        )
        .expect("write workflow yaml");
        let service = JobService::new_with_workflow_file(&db_path, &data_dir, Some(&workflow_file))
            .expect("service");

        let job_id = service
            .submit("ship feature", "resume_flow")
            .expect("submit");
        service.process_queued(10).expect("process");

        let paused = service.get_status(&job_id).expect("status").expect("row");
        assert_eq!(paused.status, "paused");
        assert_eq!(paused.paused_stage.as_deref(), Some("plan"));

        let pending = service.list_pending_decisions(None).expect("pending");
        assert_eq!(pending.len(), 1);
        service
            .approve_decision(&pending[0].id)
            .expect("approve decision");

        service.resume_job(&job_id).expect("resume");
        let done = service.get_result(&job_id).expect("result").expect("row");
        assert_eq!(done.status, "completed");
        assert!(done.outputs.iter().any(|o| o.kind == "plan-doc"));
        assert!(done.outputs.iter().any(|o| o.kind == "delivery"));
    }

    #[test]
    fn trace_pipeline_includes_persisted_discussions() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let service = JobService::new(&db_path, &data_dir).expect("service");

        let job_id = service
            .submit("build multi-agent flow", "default")
            .expect("submit");
        service.process_queued(10).expect("process");

        let status = service.get_status(&job_id).expect("status").expect("row");
        let pipeline_id = status.pipeline_id.expect("pipeline id");

        let trace = service.trace_pipeline(&pipeline_id).expect("trace");
        assert_eq!(trace.pipeline_id, pipeline_id);
        assert!(!trace.conversations.is_empty());
        assert!(!trace.conversations[0].messages.is_empty());
    }
}
