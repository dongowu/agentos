.PHONY: verify-readme test-go test-rust proto-gen

verify-readme:
	pwsh -NoProfile -Command "if ((Get-Content README.md -Raw) -match 'TODO') { exit 1 }"

test-go:
	go test ./...

test-rust:
	cd runtime; cargo test --workspace

proto-gen:
	protoc -I api/proto --go_out=api/gen --go_opt=module=github.com/agentos/agentos --go-grpc_out=api/gen --go-grpc_opt=module=github.com/agentos/agentos api/proto/agentos/v1/runtime.proto
