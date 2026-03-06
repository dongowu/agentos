package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/dongowu/ai-orchestrator/internal/bootstrap"
)

func main() {
	app, err := bootstrap.FromEnv(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	_ = app

	log.Println("controller started (orchestration loop not yet wired)")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	<-ctx.Done()
	stop()
	log.Println("controller stopped")
}
