//! AgentOS worker -- receives execution requests and runs them in sandboxes.

pub mod config;
pub mod executor;
pub mod grpc;
pub mod proto;
pub mod registration;
#[allow(deprecated)]
pub mod service;
