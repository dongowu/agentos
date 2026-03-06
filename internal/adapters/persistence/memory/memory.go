package memory

import (
	"context"
	"sync"

	"github.com/agentos/agentos/internal/adapter"
	"github.com/agentos/agentos/internal/persistence"
	"github.com/agentos/agentos/pkg/config"
	"github.com/agentos/agentos/pkg/taskdsl"
)

func init() {
	adapter.RegisterTaskRepo("memory", func(ctx context.Context, cfg config.PersistenceConfig) (persistence.TaskRepository, error) {
		return NewTaskRepository(), nil
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

var _ persistence.TaskRepository = (*TaskRepository)(nil)
