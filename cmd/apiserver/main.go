package main

import (
	"context"
	"log"
	"os"

	"github.com/dongowu/agentos/internal/access"
	"github.com/dongowu/agentos/internal/access/http"
	"github.com/dongowu/agentos/internal/agent"
	"github.com/dongowu/agentos/internal/bootstrap"
	"github.com/dongowu/agentos/internal/gateway"
	_ "github.com/dongowu/agentos/internal/tool/builtin"
)

type agentLookupAdapter struct {
	mgr *agent.Manager
}

func (a agentLookupAdapter) Get(name string) interface{ CheckPolicy(string) error } {
	if a.mgr == nil {
		return nil
	}
	return a.mgr.Get(name)
}

func (a agentLookupAdapter) List() []string {
	if a.mgr == nil {
		return []string{}
	}
	return a.mgr.List()
}

func apiListenAddr() string {
	if addr := os.Getenv("AGENTOS_API_LISTEN_ADDR"); addr != "" {
		return addr
	}
	return ":8080"
}

func main() {
	app, err := bootstrap.FromEnv(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	api := access.NewTaskSubmissionAPIImpl(app.Engine)
	gw := gateway.NewHandler(api)
	gw.AgentManager = agentLookupAdapter{mgr: app.AgentManager}

	addr := apiListenAddr()
	srv := &http.Server{Addr: addr, API: api, Audit: app.Audit, Bus: app.Bus, Gateway: gw}
	log.Printf("apiserver listening on %s", addr)
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
