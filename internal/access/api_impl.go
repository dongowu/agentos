package access

import (
	"context"
	"fmt"

	"github.com/dongowu/agentos/internal/orchestration"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

// TaskSubmissionAPIImpl implements TaskSubmissionAPI using orchestration.TaskEngine.
type TaskSubmissionAPIImpl struct {
	engine orchestration.TaskEngine
	audit  persistence.AuditLogStore
}

// NewTaskSubmissionAPIImpl returns an API backed by the given engine.
func NewTaskSubmissionAPIImpl(engine orchestration.TaskEngine) *TaskSubmissionAPIImpl {
	return &TaskSubmissionAPIImpl{engine: engine}
}

// WithAuditStore attaches the audit store used to build replay projections.
func (a *TaskSubmissionAPIImpl) WithAuditStore(store persistence.AuditLogStore) *TaskSubmissionAPIImpl {
	a.audit = store
	return a
}

// CreateTask implements TaskSubmissionAPI.
func (a *TaskSubmissionAPIImpl) CreateTask(ctx context.Context, req CreateTaskRequest) (*CreateTaskResponse, error) {
	if rich, ok := a.engine.(orchestration.RichTaskEngine); ok {
		task, err := rich.StartTaskWithInput(ctx, orchestration.StartTaskInput{
			Prompt:    req.Prompt,
			AgentName: req.AgentName,
			TenantID:  req.TenantID,
		})
		if err != nil {
			return nil, fmt.Errorf("start task: %w", err)
		}
		return &CreateTaskResponse{TaskID: task.ID, State: task.State}, nil
	}

	task, err := a.engine.StartTask(ctx, req.Prompt)
	if err != nil {
		return nil, fmt.Errorf("start task: %w", err)
	}
	return &CreateTaskResponse{TaskID: task.ID, State: task.State}, nil
}

// GetTask implements TaskSubmissionAPI.
func (a *TaskSubmissionAPIImpl) GetTask(ctx context.Context, taskID string) (*CreateTaskResponse, error) {
	task, err := a.engine.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return &CreateTaskResponse{TaskID: task.ID, State: task.State}, nil
}

// GetTaskDetail implements TaskDetailAPI.
func (a *TaskSubmissionAPIImpl) GetTaskDetail(ctx context.Context, taskID string) (*TaskDetail, error) {
	task, err := a.engine.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return &TaskDetail{TaskID: task.ID, State: task.State, TenantID: task.TenantID}, nil
}

// GetTaskReplay implements TaskReplayAPI.
func (a *TaskSubmissionAPIImpl) GetTaskReplay(ctx context.Context, taskID string) (*TaskReplay, error) {
	task, err := a.engine.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	replay := &TaskReplay{
		TaskID:    task.ID,
		State:     task.State,
		TenantID:  task.TenantID,
		AgentName: task.AgentName,
		Prompt:    task.Prompt,
		Actions:   []TaskReplayAction{},
	}

	recordsByAction := map[string]persistence.AuditRecord{}
	records := []persistence.AuditRecord{}
	if a.audit != nil {
		records, err = a.audit.ListByTask(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("list audit records: %w", err)
		}
		for _, record := range records {
			recordsByAction[record.ActionID] = record
		}
	}

	seen := map[string]struct{}{}
	if task.Plan != nil {
		for _, action := range task.Plan.Actions {
			record, ok := recordsByAction[action.ID]
			replay.Actions = append(replay.Actions, buildReplayAction(action, record, ok))
			seen[action.ID] = struct{}{}
		}
	}
	for _, record := range records {
		if _, ok := seen[record.ActionID]; ok {
			continue
		}
		replay.Actions = append(replay.Actions, buildReplayAction(taskdsl.Action{ID: record.ActionID, RuntimeEnv: record.RuntimeEnv}, record, true))
	}

	replay.Summary = summarizeReplayActions(replay.Actions)
	return replay, nil
}

func buildReplayAction(action taskdsl.Action, record persistence.AuditRecord, hasRecord bool) TaskReplayAction {
	replay := TaskReplayAction{
		ActionID:   action.ID,
		Kind:       action.Kind,
		RuntimeEnv: action.RuntimeEnv,
		Command:    actionCommand(action),
		Status:     "pending",
	}
	if !hasRecord {
		return replay
	}
	replay.Command = firstNonEmpty(record.Command, replay.Command)
	replay.RuntimeEnv = firstNonEmpty(record.RuntimeEnv, replay.RuntimeEnv)
	replay.WorkerID = record.WorkerID
	replay.Stdout = record.Stdout
	replay.Stderr = record.Stderr
	replay.Error = record.Error
	replay.SideEffects = append([]string(nil), record.SideEffects...)
	exitCode := record.ExitCode
	replay.ExitCode = &exitCode
	occurredAt := record.OccurredAt
	if !occurredAt.IsZero() {
		replay.OccurredAt = &occurredAt
	}
	if record.Error != "" || record.ExitCode != 0 {
		replay.Status = "failed"
		return replay
	}
	replay.Status = "completed"
	return replay
}

func summarizeReplayActions(actions []TaskReplayAction) TaskReplaySummary {
	summary := TaskReplaySummary{ActionCount: len(actions)}
	for _, action := range actions {
		switch action.Status {
		case "completed":
			summary.CompletedCount++
		case "failed":
			summary.CompletedCount++
			summary.FailedCount++
		}
	}
	return summary
}

func actionCommand(action taskdsl.Action) string {
	if action.Payload == nil {
		return ""
	}
	if cmd, ok := action.Payload["cmd"].(string); ok {
		return cmd
	}
	if command, ok := action.Payload["command"].(string); ok {
		return command
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
