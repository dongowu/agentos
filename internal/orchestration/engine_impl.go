package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/dongowu/agentos/internal/messaging"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/internal/policy"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/internal/scheduler"
	"github.com/dongowu/agentos/pkg/events"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

// EngineImpl implements TaskEngine.
type EngineImpl struct {
	repo          persistence.TaskRepository
	bus           messaging.EventBus
	planner       Planner
	skillResolver SkillResolver
	executor      runtimeclient.ExecutorClient // fallback for direct execution
	policy        policy.PolicyEngine          // nil = skip policy checks
	scheduler     scheduler.Scheduler          // nil = use direct executor
}

// NewEngineImpl returns a new task engine.
// skillResolver may be nil; then action.RuntimeEnv is used as profile.
// executor may be nil; then actions are only dispatched (no execution).
// pol may be nil; then policy checks are skipped.
// sched may be nil; then the direct executor path is used.
func NewEngineImpl(
	repo persistence.TaskRepository,
	bus messaging.EventBus,
	planner Planner,
	skillResolver SkillResolver,
	executor runtimeclient.ExecutorClient,
	pol policy.PolicyEngine,
	sched scheduler.Scheduler,
) *EngineImpl {
	return &EngineImpl{
		repo:          repo,
		bus:           bus,
		planner:       planner,
		skillResolver: skillResolver,
		executor:      executor,
		policy:        pol,
		scheduler:     sched,
	}
}

// StartTask creates a task, runs planning, attaches plan, and executes or dispatches actions.
func (e *EngineImpl) StartTask(ctx context.Context, prompt string) (*taskdsl.Task, error) {
	task := &taskdsl.Task{
		ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Prompt:    prompt,
		State:     string(Pending),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := e.repo.Create(ctx, task); err != nil {
		return nil, err
	}
	_ = e.bus.Publish(ctx, "task.created", &events.TaskCreated{TaskID: task.ID, Prompt: prompt, Occurred: time.Now()})

	if err := e.transition(ctx, task, Planning); err != nil {
		return task, err
	}

	plan, err := e.planner.Plan(ctx, PlanInput{TaskID: task.ID, Prompt: prompt})
	if err != nil {
		return task, fmt.Errorf("plan: %w", err)
	}
	task.Plan = plan
	task.UpdatedAt = time.Now()
	if err := e.repo.Update(ctx, task); err != nil {
		return task, err
	}
	_ = e.bus.Publish(ctx, "task.planned", &events.TaskPlanned{TaskID: task.ID, ActionCount: len(plan.Actions), Occurred: time.Now()})

	if err := e.transition(ctx, task, Queued); err != nil {
		return task, err
	}

	for i := range plan.Actions {
		action := &plan.Actions[i]
		profile := action.RuntimeEnv
		if e.skillResolver != nil {
			if p, err := e.skillResolver.Resolve(action); err == nil {
				profile = p
			}
		}
		action.RuntimeEnv = profile

		// Policy check: evaluate before dispatching each action.
		if e.policy != nil {
			decision, err := e.policy.Evaluate(ctx, policy.PolicyRequest{
				AgentName: "",
				ToolName:  action.Kind,
				Command:   extractCommand(action),
			})
			if err != nil {
				_ = e.bus.Publish(ctx, "task.action.denied", &events.ActionCompleted{TaskID: task.ID, ActionID: action.ID, ExitCode: -1, Occurred: time.Now()})
				_ = e.transition(ctx, task, Failed)
				return task, fmt.Errorf("policy error: %w", err)
			}
			if !decision.Allowed {
				_ = e.bus.Publish(ctx, "task.action.denied", &events.ActionCompleted{TaskID: task.ID, ActionID: action.ID, ExitCode: -1, Occurred: time.Now()})
				_ = e.transition(ctx, task, Failed)
				return task, fmt.Errorf("policy denied: %s", decision.Reason)
			}
		}

		_ = e.bus.Publish(ctx, "task.action.dispatched", &events.ActionDispatched{TaskID: task.ID, ActionID: action.ID, Occurred: time.Now()})

		// Scheduler path: non-blocking dispatch.
		if e.scheduler != nil {
			if err := e.transition(ctx, task, Running); err != nil {
				return task, err
			}
			if err := e.scheduler.Submit(ctx, task.ID, action); err != nil {
				_ = e.transition(ctx, task, Failed)
				return task, fmt.Errorf("scheduler submit: %w", err)
			}
			// Return immediately; ProcessResults handles completion.
			return e.repo.Get(ctx, task.ID)
		}

		// Direct executor path (fallback).
		if e.executor != nil {
			if err := e.transition(ctx, task, Running); err != nil {
				return task, err
			}
			result, err := e.executor.ExecuteAction(ctx, task.ID, action)
			if err != nil {
				_ = e.bus.Publish(ctx, "task.action.completed", &events.ActionCompleted{TaskID: task.ID, ActionID: action.ID, ExitCode: -1, Occurred: time.Now()})
				if err := e.transition(ctx, task, Failed); err != nil {
					return task, err
				}
				return e.repo.Get(ctx, task.ID)
			}
			_ = e.bus.Publish(ctx, "task.action.completed", &events.ActionCompleted{TaskID: task.ID, ActionID: action.ID, ExitCode: result.ExitCode, Occurred: time.Now()})
			if err := e.transition(ctx, task, Evaluating); err != nil {
				return task, err
			}
			if result.ExitCode != 0 {
				if err := e.transition(ctx, task, Failed); err != nil {
					return task, err
				}
				return e.repo.Get(ctx, task.ID)
			}
		}
	}

	if e.executor != nil && len(plan.Actions) > 0 {
		if err := e.transition(ctx, task, Succeeded); err != nil {
			return task, err
		}
	}

	return e.repo.Get(ctx, task.ID)
}

// ProcessResults reads completed action results from the scheduler and updates
// task state accordingly. It blocks until ctx is cancelled. Call in a goroutine.
func (e *EngineImpl) ProcessResults(ctx context.Context) {
	if e.scheduler == nil {
		return
	}
	results := e.scheduler.Results()
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-results:
			if !ok {
				return
			}
			task, err := e.repo.Get(ctx, result.TaskID)
			if err != nil || task == nil {
				continue
			}
			_ = e.bus.Publish(ctx, "task.action.completed", &events.ActionCompleted{
				TaskID:   result.TaskID,
				ActionID: result.ActionID,
				ExitCode: result.ExitCode,
				Occurred: time.Now(),
			})
			if result.ExitCode != 0 || result.Error != nil {
				_ = e.transition(ctx, task, Failed)
			} else {
				// Transition through evaluating to succeeded.
				_ = e.transition(ctx, task, Evaluating)
				_ = e.transition(ctx, task, Succeeded)
			}
		}
	}
}

func (e *EngineImpl) transition(ctx context.Context, task *taskdsl.Task, to TaskState) error {
	sm := NewTaskStateMachine()
	if _, err := sm.Transition(TaskState(task.State), to); err != nil {
		return err
	}
	task.State = string(to)
	task.UpdatedAt = time.Now()
	return e.repo.Update(ctx, task)
}

// Transition updates task state.
func (e *EngineImpl) Transition(ctx context.Context, taskID string, to TaskState) error {
	task, err := e.repo.Get(ctx, taskID)
	if err != nil || task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	sm := NewTaskStateMachine()
	_, err = sm.Transition(TaskState(task.State), to)
	if err != nil {
		return err
	}
	task.State = string(to)
	task.UpdatedAt = time.Now()
	return e.repo.Update(ctx, task)
}

// GetTask retrieves a task.
func (e *EngineImpl) GetTask(ctx context.Context, taskID string) (*taskdsl.Task, error) {
	return e.repo.Get(ctx, taskID)
}

// extractCommand pulls a command string from the action payload for policy evaluation.
func extractCommand(action *taskdsl.Action) string {
	if action.Payload == nil {
		return ""
	}
	if cmd, ok := action.Payload["cmd"]; ok {
		if s, ok := cmd.(string); ok {
			return s
		}
	}
	return ""
}
