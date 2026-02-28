mod cli;
mod core;
mod gitops;
mod guard;
mod jobs;
mod llm;
mod messaging;
mod pipeline;
mod plugins;
mod rag;
mod runtime;
mod workflow_config;

fn main() -> anyhow::Result<()> {
    cli::run()
}
