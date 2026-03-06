use agentos_worker::grpc;
use std::net::SocketAddr;
use tonic::transport::Server;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr: SocketAddr = "[::1]:50051".parse()?;
    let svc = grpc::RuntimeServiceImpl::new().into_server();

    println!("agentos-worker listening on {}", addr);
    Server::builder()
        .add_service(svc)
        .serve(addr)
        .await?;
    Ok(())
}
