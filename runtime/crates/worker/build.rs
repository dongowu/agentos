fn main() {
    // Proto types are hand-written in src/proto.rs to avoid requiring protoc.
    // If the .proto file changes, regenerate src/proto.rs manually:
    //   protoc + tonic-build, or update the structs by hand.
    //
    // To re-enable codegen, uncomment the following:
    // let proto = "../../../api/proto/agentos/v1/runtime.proto";
    // tonic_build::configure()
    //     .build_server(true)
    //     .build_client(false)
    //     .compile(&[proto], &["../../../api/proto"])
    //     .unwrap();
}
