package main

import (
	"context"
	"log"

	"github.com/dongowu/ai-orchestrator/internal/access"
	"github.com/dongowu/ai-orchestrator/internal/access/http"
	"github.com/dongowu/ai-orchestrator/internal/bootstrap"
)

func main() {
	app, err := bootstrap.FromEnv(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	api := access.NewTaskSubmissionAPIImpl(app.Engine)

	srv := &http.Server{Addr: ":8080", API: api}
	log.Println("apiserver listening on :8080")
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
