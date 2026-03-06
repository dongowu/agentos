package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	adapterruntime "github.com/dongowu/agentos/internal/adapters/runtimeclient"
	"github.com/dongowu/agentos/internal/adapters/llm/openai"
	"github.com/dongowu/agentos/internal/adapter"
	"github.com/dongowu/agentos/internal/agent"
	"github.com/dongowu/agentos/internal/memory"
	membuilder "github.com/dongowu/agentos/internal/memory/builder"
	"github.com/dongowu/agentos/internal/policy"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/internal/scheduler"
	"github.com/dongowu/agentos/internal/worker"
	"github.com/dongowu/agentos/internal/messaging"
	"github.com/dongowu/agentos/internal/orchestration"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/config"

	// Activate all built-in adapter plugins.
	_ "github.com/dongowu/agentos/internal/adapters/defaults"
	// Activate all built-in tool plugins.
	_ "github.com/dongowu/agentos/internal/tool/builtin"
)

// App holds wired dependencies.
type App struct {
	Config       config.Config
	Repo         persistence.TaskRepository
	Bus          messaging.EventBus
	Engine       orchestration.TaskEngine
	Planner      orchestration.Planner
	Resolver     orchestration.SkillResolver
	Memory       memory.Provider
	AgentManager *agent.Manager
	Policy         policy.PolicyEngine
	Vault          policy.CredentialVault
	WorkerRegistry worker.Registry
	Scheduler      scheduler.Scheduler
	closers        []io.Closer
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

	// Planner: LLM-backed or stub.
	var planner orchestration.Planner
	switch cfg.LLM.Provider {
	case "openai":
		llmClient := openai.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey)
		planner = orchestration.NewLLMPlanner(llmClient, cfg.LLM.Model)
	default:
		planner = &orchestration.StubPlanner{}
	}

	resolver := &orchestration.StubSkillResolver{}

	// Memory provider.
	memOpts := map[string]any{}
	if cfg.Memory.TTL != "" {
		memOpts["ttl"] = cfg.Memory.TTL
	}
	if cfg.Memory.Redis.Addr != "" {
		memOpts["addr"] = cfg.Memory.Redis.Addr
	}
	if cfg.Memory.Redis.Prefix != "" {
		memOpts["prefix"] = cfg.Memory.Redis.Prefix
	}
	memProv, _ := membuilder.NewProvider(cfg.Memory.Provider, memOpts)
	if memProv == nil {
		memProv, _ = membuilder.NewProvider("inmemory", nil)
	}

	// Agent manager.
	agentMgr := agent.NewManager()
	if cfg.AgentDir != "" {
		// Best-effort load; ignore errors if dir doesn't exist.
		_ = agentMgr.LoadFromDir(cfg.AgentDir)
	}

	// Policy engine.
	var policyRules []policy.Rule
	for _, r := range cfg.Policy.Rules {
		policyRules = append(policyRules, policy.Rule{
			Agent:   r.Agent,
			Actions: policy.Actions{Allow: r.Allow, Deny: r.Deny},
		})
	}
	policyEngine := policy.NewDefaultEngine(policy.Config{
		Rules:           policyRules,
		DefaultAutonomy: policy.AutonomyLevel(cfg.Policy.DefaultAutonomy),
		RateLimit:       policy.RateLimitConfig{MaxActionsPerHour: cfg.Policy.RateLimit},
	})

	// Credential vault (in-memory for MVP).
	vault := policy.NewInMemoryVault(map[string]string{})

	// Worker registry and pool.
	workerReg := worker.NewMemoryRegistry(30 * time.Second)
	workerPool := worker.NewPool(workerReg, nil) // nil dialer = default gRPC dialer

	// Scheduler.
	var sched scheduler.Scheduler
	sched = scheduler.NewLocalScheduler(workerPool)

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
	engine := orchestration.NewEngineImpl(repo, bus, planner, resolver, executor, policyEngine, sched)

	// Start background result processor for scheduler-dispatched actions.
	go engine.ProcessResults(ctx)

	app := &App{
		Config:       cfg,
		Repo:         repo,
		Bus:          bus,
		Engine:       engine,
		Planner:      planner,
		Resolver:     resolver,
		Memory:       memProv,
		AgentManager: agentMgr,
		Policy:         policyEngine,
		Vault:          vault,
		WorkerRegistry: workerReg,
		Scheduler:      sched,
		closers:        closers,
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
