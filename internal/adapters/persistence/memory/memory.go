package memory

import (
	"context"
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

func auditKey(taskID, actionID string) string {
	return taskID + "/" + actionID
}

var _ persistence.TaskRepository = (*TaskRepository)(nil)
var _ persistence.AuditLogStore = (*AuditLogStore)(nil)
