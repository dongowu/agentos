ARGS ?=

.PHONY: verify-readme test-go test-rust proto-gen dev-setup dev-up

verify-readme:
	pwsh -NoProfile -Command "if ((Get-Content README.md -Raw) -match 'TODO') { exit 1 }"

test-go:
	go test ./...

test-rust:
	cd runtime; cargo test --workspace

proto-gen:
	protoc -I api/proto --go_out=api/gen --go_opt=module=github.com/dongowu/agentos --go-grpc_out=api/gen --go-grpc_opt=module=github.com/dongowu/agentos api/proto/agentos/v1/runtime.proto

dev-setup:
	bash scripts/setup_dev_env.sh $(ARGS)

dev-up:
	bash scripts/start_full_stack.sh $(ARGS)
