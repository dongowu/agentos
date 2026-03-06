package main

import (
	"context"
	"log"

	"github.com/dongowu/ai-orchestrator/internal/access"
	"github.com/dongowu/ai-orchestrator/internal/access/cli"
	"github.com/dongowu/ai-orchestrator/internal/bootstrap"
)

func main() {
	app, err := bootstrap.FromEnv(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	api := access.NewTaskSubmissionAPIImpl(app.Engine)

	if err := cli.Root(api).Execute(); err != nil {
		log.Fatal(err)
	}
}
