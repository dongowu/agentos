use agentos_sandbox::{DockerProvider, IsolationProvider, SandboxSpec};
use std::pin::Pin;
use tonic::{Request, Response, Status};

pub mod agentos {
    tonic::include_proto!("agentos.v1");
}

use agentos::runtime_service_server::{RuntimeService, RuntimeServiceServer};
use agentos::{
    ExecuteActionRequest, ExecuteActionResponse, StreamChunk, StreamOutputRequest,
};

pub struct RuntimeServiceImpl {
    provider: DockerProvider,
}

impl RuntimeServiceImpl {
    pub fn new() -> Self {
        Self {
            provider: DockerProvider::new(),
        }
    }

    pub fn into_server(self) -> RuntimeServiceServer<Self> {
        RuntimeServiceServer::new(self)
    }
}

#[tonic::async_trait]
impl RuntimeService for RuntimeServiceImpl {
    async fn execute_action(
        &self,
        request: Request<ExecuteActionRequest>,
    ) -> Result<Response<ExecuteActionResponse>, Status> {
        let req = request.into_inner();

        let _handle = self
            .provider
            .start(SandboxSpec {
                profile: req.runtime_profile.clone(),
            })
            .await
            .map_err(|e| Status::internal(e.to_string()))?;

        Ok(Response::new(ExecuteActionResponse {
            exit_code: 0,
            stdout: b"ok\n".to_vec(),
            stderr: vec![],
        }))
    }

    type StreamOutputStream = Pin<
        Box<dyn futures::Stream<Item = Result<StreamChunk, Status>> + Send>,
    >;

    async fn stream_output(
        &self,
        request: Request<StreamOutputRequest>,
    ) -> Result<Response<Self::StreamOutputStream>, Status> {
        let req = request.into_inner();
        let stream = async_stream::stream! {
            yield Ok(StreamChunk {
                task_id: req.task_id.clone(),
                action_id: req.action_id.clone(),
                data: b"".to_vec(),
                kind: "stdout".to_string(),
            });
        };
        Ok(Response::new(Box::pin(stream)))
    }
}
