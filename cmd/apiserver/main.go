package main

import (
	"context"
	"log"

	"github.com/agentos/agentos/internal/access"
	"github.com/agentos/agentos/internal/access/http"
	"github.com/agentos/agentos/internal/bootstrap"
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
