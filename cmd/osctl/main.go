package main

import (
	"context"
	"log"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/access/cli"
	"github.com/dongowu/agentos/internal/bootstrap"
)

func main() {
	root := cli.Root(func() (access.TaskSubmissionAPI, error) {
		app, err := bootstrap.FromEnv(context.Background())
		if err != nil {
			return nil, err
		}
		return access.NewTaskSubmissionAPIImpl(app.Engine).WithAuditStore(app.Audit), nil
	})

	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}
