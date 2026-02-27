use std::sync::Arc;

use anyhow::Result;
use axum::{
    extract::{Path, State},
    http::StatusCode,
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};

use crate::jobs::{HumanDecision, JobResult, JobService, PipelineTrace};

#[derive(Clone)]
struct AppState {
    job_service: Arc<JobService>,
}

#[derive(Debug, Deserialize)]
struct SubmitRequest {
    requirement: String,
    workflow: Option<String>,
}

#[derive(Debug, Serialize)]
struct SubmitResponse {
    job_id: String,
}

#[derive(Debug, Serialize)]
struct WorkResponse {
    processed: usize,
}

async fn submit_job(
    State(state): State<AppState>,
    Json(payload): Json<SubmitRequest>,
) -> Result<Json<SubmitResponse>, StatusCode> {
    let workflow = payload.workflow.unwrap_or_else(|| "mvp".to_string());
    state
        .job_service
        .submit(&payload.requirement, &workflow)
        .map(|job_id| Json(SubmitResponse { job_id }))
        .map_err(|_e| StatusCode::BAD_REQUEST)
}

async fn process_work(
    State(state): State<AppState>,
    Json(limit): Json<usize>,
) -> Result<Json<WorkResponse>, StatusCode> {
    state
        .job_service
        .process_queued(limit)
        .map(|processed| Json(WorkResponse { processed }))
        .map_err(|_e| StatusCode::INTERNAL_SERVER_ERROR)
}

async fn get_job_status(
    State(state): State<AppState>,
    Path(job_id): Path<String>,
) -> Result<Json<JobResult>, StatusCode> {
    state
        .job_service
        .get_status(&job_id)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?
        .map(Json)
        .ok_or(StatusCode::NOT_FOUND)
}

async fn get_job_result(
    State(state): State<AppState>,
    Path(job_id): Path<String>,
) -> Result<Json<JobResult>, StatusCode> {
    state
        .job_service
        .get_result(&job_id)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?
        .map(Json)
        .ok_or(StatusCode::NOT_FOUND)
}

async fn resume_job(
    State(state): State<AppState>,
    Path(job_id): Path<String>,
) -> Result<Json<JobResult>, StatusCode> {
    state
        .job_service
        .resume_job(&job_id)
        .map_err(|_| StatusCode::BAD_REQUEST)?;
    state
        .job_service
        .get_result(&job_id)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?
        .map(Json)
        .ok_or(StatusCode::NOT_FOUND)
}

async fn list_pending_decisions(
    State(state): State<AppState>,
) -> Result<Json<Vec<HumanDecision>>, StatusCode> {
    state
        .job_service
        .list_pending_decisions(None)
        .map(Json)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)
}

async fn approve_decision(
    State(state): State<AppState>,
    Path(decision_id): Path<String>,
) -> Result<Json<serde_json::Value>, StatusCode> {
    state
        .job_service
        .approve_decision(&decision_id)
        .map_err(|_| StatusCode::BAD_REQUEST)?;
    Ok(Json(serde_json::json!({ "approved": decision_id })))
}

async fn reject_decision(
    State(state): State<AppState>,
    Path(decision_id): Path<String>,
) -> Result<Json<serde_json::Value>, StatusCode> {
    state
        .job_service
        .reject_decision(&decision_id, None)
        .map_err(|_| StatusCode::BAD_REQUEST)?;
    Ok(Json(serde_json::json!({ "rejected": decision_id })))
}

async fn get_pipeline_trace(
    State(state): State<AppState>,
    Path(pipeline_id): Path<String>,
) -> Result<Json<PipelineTrace>, StatusCode> {
    state
        .job_service
        .trace_pipeline(&pipeline_id)
        .map(Json)
        .map_err(|_| StatusCode::NOT_FOUND)
}

pub fn create_app(job_service: JobService) -> Router {
    let state = AppState {
        job_service: Arc::new(job_service),
    };
    Router::new()
        .route("/jobs", post(submit_job))
        .route("/jobs/work", post(process_work))
        .route("/jobs/{job_id}", get(get_job_status))
        .route("/jobs/{job_id}/result", get(get_job_result))
        .route("/jobs/{job_id}/resume", post(resume_job))
        .route("/decisions", get(list_pending_decisions))
        .route("/decisions/{decision_id}/approve", post(approve_decision))
        .route("/decisions/{decision_id}/reject", post(reject_decision))
        .route("/trace/{pipeline_id}", get(get_pipeline_trace))
        .with_state(state)
}

#[cfg(test)]
mod tests {
    use axum::body::Body;
    use axum::http::{Request, StatusCode};
    use tower::ServiceExt;

    use super::*;
    use crate::jobs::JobService;

    #[tokio::test]
    async fn post_jobs_returns_200_with_job_id() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let service = JobService::new(&db_path, &data_dir).expect("service");
        let app = create_app(service);

        let req = Request::builder()
            .uri("/jobs")
            .method("POST")
            .header("content-type", "application/json")
            .body(Body::from(r#"{"requirement": "test task"}"#))
            .unwrap();

        let response = app.oneshot(req).await.unwrap();
        assert_eq!(response.status(), StatusCode::OK);
    }

    #[tokio::test]
    async fn post_jobs_work_processes_queue() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let service = JobService::new(&db_path, &data_dir).expect("service");
        service.submit("test", "mvp").unwrap();
        let app = create_app(service);

        let req = Request::builder()
            .uri("/jobs/work")
            .method("POST")
            .header("content-type", "application/json")
            .body(Body::from("10"))
            .unwrap();

        let response = app.oneshot(req).await.unwrap();
        assert_eq!(response.status(), StatusCode::OK);
    }

    #[tokio::test]
    async fn get_job_status_returns_queued() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let service = JobService::new(&db_path, &data_dir).expect("service");
        let job_id = service.submit("test", "mvp").unwrap();
        let app = create_app(service);

        let req = Request::builder()
            .uri(format!("/jobs/{}", job_id))
            .method("GET")
            .body(Body::empty())
            .unwrap();

        let response = app.oneshot(req).await.unwrap();
        assert_eq!(response.status(), StatusCode::OK);
    }

    #[tokio::test]
    async fn post_job_resume_paused_job() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let service = JobService::new(&db_path, &data_dir).expect("service");

        let job_id = service.submit("[[decisions:escalate]] test", "autonomy").unwrap();
        service.process_queued(10).unwrap();

        let pending = service.list_pending_decisions(None).unwrap();
        assert!(!pending.is_empty());
        service.approve_decision(&pending[0].id).unwrap();

        let app = create_app(service);

        let req = Request::builder()
            .uri(format!("/jobs/{}/resume", job_id))
            .method("POST")
            .body(Body::empty())
            .unwrap();

        let response = app.oneshot(req).await.unwrap();
        assert_eq!(response.status(), StatusCode::OK);
    }

    #[tokio::test]
    async fn get_trace_pipeline() {
        let temp = tempfile::tempdir().expect("tempdir");
        let db_path = temp.path().join("orchestrator.db");
        let data_dir = temp.path().join("data");
        let service = JobService::new(&db_path, &data_dir).expect("service");

        let job_id = service.submit("test", "mvp").unwrap();
        service.process_queued(10).unwrap();

        let status = service.get_status(&job_id).unwrap().unwrap();
        let pipeline_id = status.pipeline_id.unwrap();

        let app = create_app(service);

        let req = Request::builder()
            .uri(format!("/trace/{}", pipeline_id))
            .method("GET")
            .body(Body::empty())
            .unwrap();

        let response = app.oneshot(req).await.unwrap();
        assert_eq!(response.status(), StatusCode::OK);
    }
}
