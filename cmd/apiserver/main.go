package main

import (
	"context"
	"log"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/access/http"
	"github.com/dongowu/agentos/internal/bootstrap"
	"github.com/dongowu/agentos/internal/gateway"
	_ "github.com/dongowu/agentos/internal/tool/builtin"
)

func main() {
	app, err := bootstrap.FromEnv(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	api := access.NewTaskSubmissionAPIImpl(app.Engine)
	gw := gateway.NewHandler(api)

	srv := &http.Server{Addr: ":8080", API: api, Gateway: gw}
	log.Println("apiserver listening on :8080")
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
