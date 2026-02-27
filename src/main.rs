mod cli;
mod gitops;
mod guard;
mod jobs;
mod messaging;
mod pipeline;
mod rag;
mod workflow_config;

fn main() -> anyhow::Result<()> {
    cli::run()
}
