package main

import (
	"context"
	"log"

	"github.com/agentos/agentos/internal/access"
	"github.com/agentos/agentos/internal/access/cli"
	"github.com/agentos/agentos/internal/bootstrap"
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
