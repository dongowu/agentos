package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/agentos/agentos/internal/messaging"
	"github.com/agentos/agentos/internal/persistence"
	"github.com/agentos/agentos/internal/runtimeclient"
	"github.com/agentos/agentos/pkg/events"
	"github.com/agentos/agentos/pkg/taskdsl"
)

// EngineImpl implements TaskEngine.
type EngineImpl struct {
	repo          persistence.TaskRepository
	bus           messaging.EventBus
	planner       Planner
	skillResolver SkillResolver
	executor      runtimeclient.ExecutorClient
}

// NewEngineImpl returns a new task engine.
// skillResolver may be nil; then action.RuntimeEnv is used as profile.
// executor may be nil; then actions are only dispatched (no execution).
func NewEngineImpl(repo persistence.TaskRepository, bus messaging.EventBus, planner Planner, skillResolver SkillResolver, executor runtimeclient.ExecutorClient) *EngineImpl {
	return &EngineImpl{repo: repo, bus: bus, planner: planner, skillResolver: skillResolver, executor: executor}
}

// StartTask creates a task, runs planning, attaches plan, and transitions to Queued.
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
		_ = e.bus.Publish(ctx, "task.action.dispatched", &events.ActionDispatched{TaskID: task.ID, ActionID: action.ID, Occurred: time.Now()})

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
