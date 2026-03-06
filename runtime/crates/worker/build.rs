fn main() -> Result<(), Box<dyn std::error::Error>> {
    let proto = "../../../api/proto/agentos/v1/runtime.proto";
    tonic_build::configure()
        .build_server(true)
        .build_client(false)
        .compile(&[proto], &["../../../api/proto"])?;
    Ok(())
}
