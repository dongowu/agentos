package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"

	adapterruntime "github.com/dongowu/agentos/internal/adapters/runtimeclient"
	"github.com/dongowu/agentos/internal/adapter"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/internal/messaging"
	"github.com/dongowu/agentos/internal/orchestration"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/config"

	// Activate all built-in adapter plugins.
	_ "github.com/dongowu/agentos/internal/adapters/defaults"
)

// App holds wired dependencies.
type App struct {
	Config   config.Config
	Repo     persistence.TaskRepository
	Bus      messaging.EventBus
	Engine   orchestration.TaskEngine
	Planner  orchestration.Planner
	Resolver orchestration.SkillResolver
	closers  []io.Closer
}

// New builds an App from config.
// Use FromEnv to load config from AGENTOS_MODE (dev=memory adapters, else nats+postgres).
func New(ctx context.Context, cfg config.Config) (*App, error) {
	bus, err := adapter.NewEventBus(cfg.Messaging)
	if err != nil {
		return nil, fmt.Errorf("event bus: %w", err)
	}

	repo, err := adapter.NewTaskRepo(ctx, cfg.Persistence)
	if err != nil {
		return nil, fmt.Errorf("task repository: %w", err)
	}

	planner := &orchestration.StubPlanner{}
	resolver := &orchestration.StubSkillResolver{}
	var executor runtimeclient.ExecutorClient
	var closers []io.Closer
	if addr := cfg.Runtime.WorkerAddr; addr != "" {
		ec, err := adapterruntime.NewGRPCExecutorClient(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("executor client: %w", err)
		}
		executor = ec
		closers = append(closers, ec)
	}
	engine := orchestration.NewEngineImpl(repo, bus, planner, resolver, executor)

	app := &App{
		Config:   cfg,
		Repo:     repo,
		Bus:      bus,
		Engine:   engine,
		Planner:  planner,
		Resolver: resolver,
		closers:  closers,
	}
	if c, ok := bus.(io.Closer); ok {
		app.closers = append(app.closers, c)
	}
	if c, ok := repo.(io.Closer); ok {
		app.closers = append(app.closers, c)
	}
	return app, nil
}

// Close releases resources held by adapters (e.g. NATS connection, Postgres pool).
func (a *App) Close() error {
	var firstErr error
	for _, c := range a.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// FromEnv builds an App from environment.
// AGENTOS_MODE=dev uses memory adapters; otherwise uses NATS + Postgres.
func FromEnv(ctx context.Context) (*App, error) {
	cfg := config.Default()
	if os.Getenv("AGENTOS_MODE") == "dev" {
		cfg = config.Dev()
	}
	return New(ctx, cfg)
}
