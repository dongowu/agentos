package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dongowu/agentos/internal/adapter"
	"github.com/dongowu/agentos/internal/adapters/llm"
	"github.com/dongowu/agentos/internal/adapters/llm/openai"
	adapternats "github.com/dongowu/agentos/internal/adapters/messaging/nats"
	adapterruntime "github.com/dongowu/agentos/internal/adapters/runtimeclient"
	"github.com/dongowu/agentos/internal/agent"
	"github.com/dongowu/agentos/internal/memory"
	membuilder "github.com/dongowu/agentos/internal/memory/builder"
	"github.com/dongowu/agentos/internal/messaging"
	"github.com/dongowu/agentos/internal/orchestration"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/internal/policy"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/internal/scheduler"
	"github.com/dongowu/agentos/internal/worker"
	"github.com/dongowu/agentos/pkg/config"

	_ "github.com/dongowu/agentos/internal/adapters/defaults"
	_ "github.com/dongowu/agentos/internal/tool/builtin"
)

// App holds wired dependencies.
type App struct {
	Config         config.Config
	Repo           persistence.TaskRepository
	Bus            messaging.EventBus
	Engine         orchestration.TaskEngine
	Planner        orchestration.Planner
	Resolver       orchestration.SkillResolver
	Memory         memory.Provider
	AgentManager   *agent.Manager
	Policy         policy.PolicyEngine
	Vault          policy.CredentialVault
	WorkerRegistry worker.Registry
	Scheduler      scheduler.Scheduler
	closers        []io.Closer
}

type closeFunc func() error

func (f closeFunc) Close() error { return f() }

func llmProviderFromConfig(cfg config.Config) (llm.Provider, string) {
	if cfg.LLM.Provider != "openai" || cfg.LLM.APIKey == "" {
		return nil, ""
	}
	baseURL := cfg.LLM.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	model := cfg.LLM.Model
	if model == "" {
		model = "gpt-4o"
	}
	return openai.NewClient(baseURL, cfg.LLM.APIKey), model
}

func plannerFromConfig(cfg config.Config) orchestration.Planner {
	promptPlanner := &orchestration.PromptPlanner{}
	provider, model := llmProviderFromConfig(cfg)
	if provider == nil {
		return promptPlanner
	}
	llmPlanner := orchestration.NewRetryPlanner(orchestration.NewLLMPlanner(provider, model), 2)
	return orchestration.NewFallbackPlanner(llmPlanner, promptPlanner)
}

func schedulerFromConfig(ctx context.Context, cfg config.Config, pool scheduler.WorkerPool, startDispatcher bool) (scheduler.Scheduler, []io.Closer, error) {
	switch cfg.Scheduler.Mode {
	case "", "local":
		sched := scheduler.NewLocalScheduler(pool)
		return sched, []io.Closer{sched}, nil
	case "nats":
		nc, js, stream, err := adapternats.OpenJetStream(cfg.Messaging.NATS.URL, cfg.Messaging.NATS.Stream)
		if err != nil {
			return nil, nil, fmt.Errorf("open jetstream: %w", err)
		}
		sched := scheduler.NewNATSSchedulerFromJetStream(js, stream)
		closers := []io.Closer{closeFunc(func() error {
			nc.Close()
			return nil
		}), sched}
		if startDispatcher {
			dispatcher := scheduler.NewNATSDispatcherFromJetStream(js, stream, pool)
			if err := dispatcher.Start(ctx); err != nil {
				nc.Close()
				return nil, nil, fmt.Errorf("start dispatcher: %w", err)
			}
			closers = append(closers, dispatcher)
		}
		return sched, closers, nil
	default:
		return nil, nil, fmt.Errorf("unknown scheduler mode %q", cfg.Scheduler.Mode)
	}
}

// New builds an App from config.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	bus, err := adapter.NewEventBus(cfg.Messaging)
	if err != nil {
		return nil, fmt.Errorf("event bus: %w", err)
	}

	repo, err := adapter.NewTaskRepo(ctx, cfg.Persistence)
	if err != nil {
		return nil, fmt.Errorf("task repository: %w", err)
	}

	planner := plannerFromConfig(cfg)
	resolver := &orchestration.StubSkillResolver{}

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
	memoryHook := orchestration.NewMemoryHook(memProv)

	agentMgr := agent.NewManager()
	if cfg.AgentDir != "" {
		_ = agentMgr.LoadFromDir(cfg.AgentDir)
	}

	var policyRules []policy.Rule
	for _, r := range cfg.Policy.Rules {
		policyRules = append(policyRules, policy.Rule{Agent: r.Agent, Actions: policy.Actions{Allow: r.Allow, Deny: r.Deny}})
	}
	policyEngine := policy.NewDefaultEngine(policy.Config{
		Rules:           policyRules,
		DefaultAutonomy: policy.AutonomyLevel(cfg.Policy.DefaultAutonomy),
		RateLimit:       policy.RateLimitConfig{MaxActionsPerHour: cfg.Policy.RateLimit},
	})

	vault := policy.NewInMemoryVault(cfg.Vault.AgentSecrets)

	heartbeatTimeout := durationOrDefault(cfg.Scheduler.HeartbeatTimeout, 30*time.Second)
	var closers []io.Closer
	var workerReg worker.Registry
	if cfg.Scheduler.ControlPlaneAddr != "" {
		remoteReg, err := worker.NewRemoteRegistry(ctx, cfg.Scheduler.ControlPlaneAddr)
		if err != nil {
			return nil, fmt.Errorf("worker registry: %w", err)
		}
		workerReg = remoteReg
		closers = append(closers, remoteReg)
	} else {
		workerReg = worker.NewMemoryRegistry(heartbeatTimeout)
	}
	workerPool := worker.NewPool(workerReg, worker.NewGRPCDialer())
	closers = append(closers, workerPool)

	startDispatcher := cfg.Scheduler.Mode == "nats" && cfg.Scheduler.ControlPlaneAddr == ""
	sched, schedClosers, err := schedulerFromConfig(ctx, cfg, workerPool, startDispatcher)
	if err != nil {
		return nil, fmt.Errorf("scheduler: %w", err)
	}
	closers = append(closers, schedClosers...)

	var executor runtimeclient.ExecutorClient
	if addr := cfg.Runtime.WorkerAddr; addr != "" {
		ec, err := adapterruntime.NewGRPCExecutorClient(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("executor client: %w", err)
		}
		executor = ec
		closers = append(closers, ec)
	}
	engine := orchestration.NewEngineImpl(repo, bus, planner, resolver, executor, policyEngine, sched).
		WithMemoryHook(memoryHook).
		WithVault(vault)

	go engine.ProcessResults(ctx)

	app := &App{
		Config:         cfg,
		Repo:           repo,
		Bus:            bus,
		Engine:         engine,
		Planner:        planner,
		Resolver:       resolver,
		Memory:         memProv,
		AgentManager:   agentMgr,
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
func FromEnv(ctx context.Context) (*App, error) {
	cfg := config.Default()
	if os.Getenv("AGENTOS_MODE") == "dev" {
		cfg = config.Dev()
	}
	cfg = config.ApplyEnvOverrides(cfg)
	return New(ctx, cfg)
}

func durationOrDefault(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}
