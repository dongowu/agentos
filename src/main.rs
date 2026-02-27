mod cli;
mod gitops;
mod guard;
mod jobs;
mod messaging;
mod pipeline;
mod rag;
mod server;
mod workflow_config;

fn main() -> anyhow::Result<()> {
    tokio::runtime::Runtime::new()?.block_on(async {
        cli::run().await
    })
}
