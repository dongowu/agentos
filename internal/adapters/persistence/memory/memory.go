package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/dongowu/agentos/internal/adapter"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/config"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

func init() {
	adapter.RegisterTaskRepo("memory", func(ctx context.Context, cfg config.PersistenceConfig) (persistence.TaskRepository, error) {
		return NewTaskRepository(), nil
	})
	adapter.RegisterAuditLogStore("memory", func(ctx context.Context, cfg config.PersistenceConfig) (persistence.AuditLogStore, error) {
		return NewAuditLogStore(), nil
	})
}

// TaskRepository is an in-memory implementation of persistence.TaskRepository.
type TaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]*taskdsl.Task
}

// NewTaskRepository returns a new in-memory task repository.
func NewTaskRepository() *TaskRepository {
	return &TaskRepository{tasks: make(map[string]*taskdsl.Task)}
}

// Create implements persistence.TaskRepository.
func (r *TaskRepository) Create(ctx context.Context, task *taskdsl.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task
	return nil
}

// Get implements persistence.TaskRepository.
func (r *TaskRepository) Get(ctx context.Context, id string) (*taskdsl.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, nil
	}
	return t, nil
}

// Update implements persistence.TaskRepository.
func (r *TaskRepository) Update(ctx context.Context, task *taskdsl.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task
	return nil
}

// ListRecoverable returns queued and running tasks in creation order.
func (r *TaskRepository) ListRecoverable(ctx context.Context) ([]*taskdsl.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*taskdsl.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		switch task.State {
		case "queued", "running":
			out = append(out, task)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// AuditLogStore is an in-memory implementation of persistence.AuditLogStore.
type AuditLogStore struct {
	mu      sync.RWMutex
	records map[string]persistence.AuditRecord
	byTask  map[string][]string
}

// NewAuditLogStore returns a new in-memory audit log store.
func NewAuditLogStore() *AuditLogStore {
	return &AuditLogStore{
		records: make(map[string]persistence.AuditRecord),
		byTask:  make(map[string][]string),
	}
}

// Append implements persistence.AuditLogStore.
func (s *AuditLogStore) Append(ctx context.Context, record persistence.AuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := auditKey(record.TaskID, record.ActionID)
	if _, exists := s.records[key]; !exists {
		s.byTask[record.TaskID] = append(s.byTask[record.TaskID], key)
	}
	s.records[key] = record
	return nil
}

// Get implements persistence.AuditLogStore.
func (s *AuditLogStore) Get(ctx context.Context, taskID, actionID string) (*persistence.AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[auditKey(taskID, actionID)]
	if !ok {
		return nil, nil
	}
	copyRecord := record
	return &copyRecord, nil
}

// ListByTask implements persistence.AuditLogStore.
func (s *AuditLogStore) ListByTask(ctx context.Context, taskID string) ([]persistence.AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := s.byTask[taskID]
	out := make([]persistence.AuditRecord, 0, len(keys))
	for _, key := range keys {
		out = append(out, s.records[key])
	}
	return out, nil
}

// Query implements persistence.AuditLogStore.
func (s *AuditLogStore) Query(ctx context.Context, query persistence.AuditQuery) ([]persistence.AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]persistence.AuditRecord, 0, len(s.records))
	for _, record := range s.records {
		if !matchAuditQuery(record, query) {
			continue
		}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OccurredAt.Equal(out[j].OccurredAt) {
			if out[i].TaskID == out[j].TaskID {
				return out[i].ActionID > out[j].ActionID
			}
			return out[i].TaskID > out[j].TaskID
		}
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})
	if query.Limit > 0 && len(out) > query.Limit {
		out = out[:query.Limit]
	}
	return out, nil
}

func matchAuditQuery(record persistence.AuditRecord, query persistence.AuditQuery) bool {
	if query.TaskID != "" && record.TaskID != query.TaskID {
		return false
	}
	if query.ActionID != "" && record.ActionID != query.ActionID {
		return false
	}
	if query.TenantID != "" && record.TenantID != query.TenantID {
		return false
	}
	if query.AgentName != "" && record.AgentName != query.AgentName {
		return false
	}
	if query.WorkerID != "" && record.WorkerID != query.WorkerID {
		return false
	}
	if query.FailedOnly && record.Error == "" && record.ExitCode == 0 {
		return false
	}
	return true
}

func auditKey(taskID, actionID string) string {
	return taskID + "/" + actionID
}

var _ persistence.TaskRepository = (*TaskRepository)(nil)
var _ persistence.AuditLogStore = (*AuditLogStore)(nil)
