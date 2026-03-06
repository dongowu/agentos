package main

import (
	"context"
	"log"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/access/cli"
	"github.com/dongowu/agentos/internal/bootstrap"
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
