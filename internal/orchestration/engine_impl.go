package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dongowu/agentos/internal/adapters/llm"
	"github.com/dongowu/agentos/internal/memory"
	"github.com/dongowu/agentos/internal/messaging"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/internal/policy"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/internal/scheduler"
	"github.com/dongowu/agentos/internal/tool"
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
	memoryHook    *MemoryHook
	vault         policy.CredentialVault
	auditStore    persistence.AuditLogStore
	llmProvider   llm.Provider // nil = use legacy plan+execute path
	llmModel      string
	tools         []tool.Tool // tools available for agent loop
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

// WithMemoryHook attaches a memory hook to the engine.
func (e *EngineImpl) WithMemoryHook(hook *MemoryHook) *EngineImpl {
	e.memoryHook = hook
	return e
}

// WithVault attaches a credential vault to the engine.
func (e *EngineImpl) WithVault(vault policy.CredentialVault) *EngineImpl {
	e.vault = vault
	return e
}

// WithAuditStore attaches an audit store to the engine.
func (e *EngineImpl) WithAuditStore(store persistence.AuditLogStore) *EngineImpl {
	e.auditStore = store
	return e
}

// StartTask creates a task, runs planning, attaches plan, and executes or dispatches actions.
func (e *EngineImpl) StartTask(ctx context.Context, prompt string) (*taskdsl.Task, error) {
	return e.StartTaskWithInput(ctx, StartTaskInput{Prompt: prompt})
}

// StartTaskWithInput creates a task with richer execution context.
func (e *EngineImpl) StartTaskWithInput(ctx context.Context, input StartTaskInput) (*taskdsl.Task, error) {
	task := &taskdsl.Task{
		ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Prompt:    input.Prompt,
		State:     string(Pending),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := e.repo.Create(ctx, task); err != nil {
		return nil, err
	}
	_ = e.bus.Publish(ctx, "task.created", &events.TaskCreated{TaskID: task.ID, Prompt: input.Prompt, Occurred: time.Now()})

	// Agent loop path: when an LLM provider is configured, use the multi-turn
	// tool-calling loop instead of the legacy plan+execute path.
	if e.llmProvider != nil {
		return e.runAgentLoop(ctx, task)
	}

	if err := e.transition(ctx, task, Planning); err != nil {
		return task, err
	}

	planningPrompt := input.Prompt
	if e.memoryHook != nil {
		if recalled, err := e.memoryHook.RecallContext(ctx, input.Prompt, 3); err == nil && len(recalled) > 0 {
			planningPrompt = augmentPlanningPrompt(input.Prompt, recalled)
		}
	}

	plan, err := e.planner.Plan(ctx, PlanInput{TaskID: task.ID, Prompt: planningPrompt, TenantID: input.TenantID})
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

		injectCredentialToken(ctx, e.vault, input.AgentName, action)

		// Policy check: evaluate before dispatching each action.
		if e.policy != nil {
			decision, err := e.policy.Evaluate(ctx, policy.PolicyRequest{
				AgentName: input.AgentName,
				ToolName:  action.Kind,
				Command:   extractCommand(action),
				TenantID:  input.TenantID,
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
				if e.executor != nil {
					result, execErr := e.executeDirect(ctx, task, action)
					if execErr != nil {
						return task, fmt.Errorf("scheduler submit: %w; direct execute: %w", err, execErr)
					}
					return result, nil
				}
				_ = e.transition(ctx, task, Failed)
				return task, fmt.Errorf("scheduler submit: %w", err)
			}
			// Return immediately; ProcessResults handles completion.
			return e.repo.Get(ctx, task.ID)
		}

		// Direct executor path (fallback).
		if e.executor != nil {
			result, err := e.executeDirect(ctx, task, action)
			if err != nil {
				return task, err
			}
			return result, nil
		}
	}

	if e.executor != nil && len(plan.Actions) > 0 {
		return e.repo.Get(ctx, task.ID)
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
			completed := &events.ActionCompleted{
				TaskID:   result.TaskID,
				ActionID: result.ActionID,
				ExitCode: result.ExitCode,
				Stdout:   string(result.Stdout),
				Stderr:   string(result.Stderr),
				WorkerID: result.WorkerID,
				Occurred: time.Now(),
			}
			if result.Error != nil {
				completed.Error = result.Error.Error()
			}
			_ = e.bus.Publish(ctx, "task.action.completed", completed)
			action := findTaskAction(task, result.ActionID)
			e.appendAudit(ctx, task, action, &result, completed.Error)
			if result.ExitCode != 0 || result.Error != nil {
				_ = e.transition(ctx, task, Failed)
			} else {
				// Transition through evaluating to succeeded.
				_ = e.transition(ctx, task, Evaluating)
				_ = e.transition(ctx, task, Succeeded)
			}
			if e.memoryHook != nil {
				_ = e.memoryHook.StoreResult(ctx, result.TaskID, map[string]any{
					"action_id": result.ActionID,
					"exit_code": result.ExitCode,
					"stdout":    string(result.Stdout),
					"stderr":    string(result.Stderr),
					"worker_id": result.WorkerID,
				})
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

func (e *EngineImpl) executeDirect(ctx context.Context, task *taskdsl.Task, action *taskdsl.Action) (*taskdsl.Task, error) {
	if TaskState(task.State) != Running {
		if err := e.transition(ctx, task, Running); err != nil {
			return task, err
		}
	}
	var (
		result *runtimeclient.ExecutionResult
		err    error
	)
	if streamer, ok := e.executor.(runtimeclient.StreamingExecutorClient); ok {
		result, err = streamer.ExecuteStream(ctx, task.ID, action, func(chunk runtimeclient.StreamChunk) {
			e.publishActionOutput(ctx, chunk)
		})
	} else {
		result, err = e.executor.ExecuteAction(ctx, task.ID, action)
		if err == nil && result != nil {
			if len(result.Stdout) > 0 {
				e.publishActionOutput(ctx, runtimeclient.StreamChunk{TaskID: task.ID, ActionID: action.ID, Kind: "stdout", Data: result.Stdout})
			}
			if len(result.Stderr) > 0 {
				e.publishActionOutput(ctx, runtimeclient.StreamChunk{TaskID: task.ID, ActionID: action.ID, Kind: "stderr", Data: result.Stderr})
			}
		}
	}
	if err != nil {
		completed := &events.ActionCompleted{TaskID: task.ID, ActionID: action.ID, ExitCode: -1, Error: err.Error(), Occurred: time.Now()}
		_ = e.bus.Publish(ctx, "task.action.completed", completed)
		e.appendAudit(ctx, task, action, &scheduler.ActionResult{TaskID: task.ID, ActionID: action.ID, ExitCode: -1, Error: err}, err.Error())
		if err := e.transition(ctx, task, Failed); err != nil {
			return task, err
		}
		return e.repo.Get(ctx, task.ID)
	}
	completed := &events.ActionCompleted{TaskID: task.ID, ActionID: action.ID, ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr), Occurred: time.Now()}
	_ = e.bus.Publish(ctx, "task.action.completed", completed)
	e.appendAudit(ctx, task, action, &scheduler.ActionResult{TaskID: task.ID, ActionID: action.ID, ExitCode: result.ExitCode, Stdout: result.Stdout, Stderr: result.Stderr}, "")
	if err := e.transition(ctx, task, Evaluating); err != nil {
		return task, err
	}
	if result.ExitCode != 0 {
		if err := e.transition(ctx, task, Failed); err != nil {
			return task, err
		}
		if e.memoryHook != nil {
			_ = e.memoryHook.StoreResult(ctx, task.ID, map[string]any{
				"action_id": action.ID,
				"exit_code": result.ExitCode,
				"stdout":    string(result.Stdout),
				"stderr":    string(result.Stderr),
			})
		}
		return e.repo.Get(ctx, task.ID)
	}
	if err := e.transition(ctx, task, Succeeded); err != nil {
		return task, err
	}
	if e.memoryHook != nil {
		_ = e.memoryHook.StoreResult(ctx, task.ID, map[string]any{
			"action_id": action.ID,
			"exit_code": result.ExitCode,
			"stdout":    string(result.Stdout),
			"stderr":    string(result.Stderr),
		})
	}
	return e.repo.Get(ctx, task.ID)
}

func (e *EngineImpl) publishActionOutput(ctx context.Context, chunk runtimeclient.StreamChunk) {
	if e.bus == nil {
		return
	}
	_ = e.bus.Publish(ctx, "task.action.output", &events.ActionOutputChunk{
		TaskID:   chunk.TaskID,
		ActionID: chunk.ActionID,
		Kind:     chunk.Kind,
		Data:     append([]byte(nil), chunk.Data...),
		Text:     string(chunk.Data),
		Occurred: time.Now(),
	})
}

func (e *EngineImpl) appendAudit(ctx context.Context, task *taskdsl.Task, action *taskdsl.Action, result *scheduler.ActionResult, errorText string) {
	if e.auditStore == nil || task == nil || result == nil {
		return
	}
	command := ""
	runtimeEnv := ""
	if action != nil {
		command = extractCommand(action)
		runtimeEnv = action.RuntimeEnv
	}
	_ = e.auditStore.Append(ctx, persistence.AuditRecord{
		TaskID:      task.ID,
		ActionID:    result.ActionID,
		Command:     command,
		RuntimeEnv:  runtimeEnv,
		WorkerID:    result.WorkerID,
		ExitCode:    result.ExitCode,
		Stdout:      string(result.Stdout),
		Stderr:      string(result.Stderr),
		Error:       errorText,
		SideEffects: nil,
		OccurredAt:  time.Now(),
	})
}

func findTaskAction(task *taskdsl.Task, actionID string) *taskdsl.Action {
	if task == nil || task.Plan == nil {
		return nil
	}
	for i := range task.Plan.Actions {
		if task.Plan.Actions[i].ID == actionID {
			return &task.Plan.Actions[i]
		}
	}
	return nil
}

func augmentPlanningPrompt(prompt string, recalled []memory.SearchResult) string {
	if len(recalled) == 0 {
		return prompt
	}
	contextBlock := "\n\nRelevant past context:\n"
	for _, item := range recalled {
		contextBlock += "- " + string(item.Content) + "\n"
	}
	return prompt + contextBlock
}

func injectCredentialToken(ctx context.Context, vault policy.CredentialVault, agentName string, action *taskdsl.Action) {
	if vault == nil || agentName == "" || action == nil {
		return
	}
	token, err := vault.GetToken(ctx, agentName)
	if err != nil || token == "" {
		return
	}
	if action.Payload == nil {
		action.Payload = map[string]any{}
	}
	env, ok := action.Payload["env"].(map[string]any)
	if !ok || env == nil {
		env = map[string]any{}
	}
	env["AGENTOS_CREDENTIAL_TOKEN"] = token
	action.Payload["env"] = env
}

// extractCommand pulls a command string from the action payload for policy evaluation.
func extractCommand(action *taskdsl.Action) string {
	if action == nil || action.Payload == nil {
		return ""
	}
	if cmd, ok := action.Payload["cmd"]; ok {
		if s, ok := cmd.(string); ok {
			return s
		}
	}
	if cmd, ok := action.Payload["command"]; ok {
		if s, ok := cmd.(string); ok {
			return s
		}
	}
	return ""
}

const maxAgentLoopIterations = 10

const agentSystemPrompt = `You are an AI agent running on AgentOS. You have access to tools that you can call to accomplish tasks.

When you need to perform an action, use the available tools. When you have completed the task or have the final answer, respond with your conclusion in plain text without calling any tools.

Be concise and focused. Execute the minimum number of tool calls needed to accomplish the task.`

// WithLLMProvider attaches an LLM provider for agent loop mode.
func (e *EngineImpl) WithLLMProvider(provider llm.Provider, model string) *EngineImpl {
	e.llmProvider = provider
	e.llmModel = model
	return e
}

// WithTools sets the tools available for the agent loop.
func (e *EngineImpl) WithTools(tools []tool.Tool) *EngineImpl {
	e.tools = tools
	return e
}

// runAgentLoop executes the multi-turn tool-calling loop.
func (e *EngineImpl) runAgentLoop(ctx context.Context, task *taskdsl.Task) (*taskdsl.Task, error) {
	if err := e.transition(ctx, task, Planning); err != nil {
		return task, err
	}
	if err := e.transition(ctx, task, Queued); err != nil {
		return task, err
	}
	if err := e.transition(ctx, task, Running); err != nil {
		return task, err
	}

	toolDefs := tool.BuildToolDefs(e.tools)
	toolMap := make(map[string]tool.Tool, len(e.tools))
	for _, t := range e.tools {
		toolMap[t.Name()] = t
	}

	messages := []llm.Message{
		{Role: "system", Content: agentSystemPrompt},
		{Role: "user", Content: task.Prompt},
	}

	for i := 0; i < maxAgentLoopIterations; i++ {
		resp, err := e.llmProvider.Chat(ctx, llm.Request{
			Model:       e.llmModel,
			Messages:    messages,
			Temperature: 0.2,
			Tools:       toolDefs,
		})
		if err != nil {
			_ = e.transition(ctx, task, Failed)
			return task, fmt.Errorf("agent loop iteration %d: %w", i, err)
		}

		_ = e.bus.Publish(ctx, "task.loop.iteration", &events.LoopIteration{
			TaskID:    task.ID,
			Iteration: i + 1,
			ToolCalls: len(resp.ToolCalls),
			Occurred:  time.Now(),
		})

		// No tool calls: LLM is done, this is the final answer.
		if len(resp.ToolCalls) == 0 {
			task.Result = resp.Content
			task.UpdatedAt = time.Now()
			_ = e.repo.Update(ctx, task)
			_ = e.transition(ctx, task, Evaluating)
			_ = e.transition(ctx, task, Succeeded)
			return e.repo.Get(ctx, task.ID)
		}

		// Append assistant message with tool calls.
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and collect results.
		for _, tc := range resp.ToolCalls {
			actionID := fmt.Sprintf("loop-%d-%s", i+1, tc.ID)

			_ = e.bus.Publish(ctx, "task.action.dispatched", &events.ActionDispatched{
				TaskID:   task.ID,
				ActionID: actionID,
				Occurred: time.Now(),
			})

			t, ok := toolMap[tc.Name]
			if !ok {
				messages = append(messages, llm.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("error: tool %q not found", tc.Name),
				})
				continue
			}

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
				messages = append(messages, llm.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("error: invalid arguments: %v", err),
				})
				continue
			}

			result, err := t.Run(ctx, args)
			var resultStr string
			if err != nil {
				resultStr = fmt.Sprintf("error: %v", err)
			} else {
				resultBytes, _ := json.Marshal(result)
				resultStr = string(resultBytes)
			}

			_ = e.bus.Publish(ctx, "task.action.completed", &events.ActionCompleted{
				TaskID:   task.ID,
				ActionID: actionID,
				ExitCode: 0,
				Stdout:   resultStr,
				Occurred: time.Now(),
			})

			messages = append(messages, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    resultStr,
			})
		}
	}

	// Max iterations reached.
	task.Result = "agent loop exceeded maximum iterations"
	task.UpdatedAt = time.Now()
	_ = e.repo.Update(ctx, task)
	_ = e.transition(ctx, task, Failed)
	return e.repo.Get(ctx, task.ID)
}
