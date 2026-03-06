package postgres

import (
	"context"
	"encoding/json"

	"github.com/dongowu/ai-orchestrator/internal/adapter"
	"github.com/dongowu/ai-orchestrator/internal/persistence"
	"github.com/dongowu/ai-orchestrator/pkg/config"
	"github.com/dongowu/ai-orchestrator/pkg/taskdsl"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func init() {
	adapter.RegisterTaskRepo("postgres", func(ctx context.Context, cfg config.PersistenceConfig) (persistence.TaskRepository, error) {
		dsn := cfg.Postgres.DSN
		if dsn == "" {
			dsn = "postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable"
		}
		return NewTaskRepository(ctx, dsn)
	})
}

// TaskRepository is a PostgreSQL implementation of persistence.TaskRepository.
type TaskRepository struct {
	pool *pgxpool.Pool
}

// NewTaskRepository creates tables and returns a Postgres task repository.
func NewTaskRepository(ctx context.Context, dsn string) (*TaskRepository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &TaskRepository{pool: pool}, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			prompt TEXT NOT NULL,
			state TEXT NOT NULL,
			plan_json JSONB,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)
	`)
	return err
}

// Create implements persistence.TaskRepository.
func (r *TaskRepository) Create(ctx context.Context, task *taskdsl.Task) error {
	planJSON, err := json.Marshal(task.Plan)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO tasks (id, prompt, state, plan_json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		task.ID, task.Prompt, task.State, planJSON, task.CreatedAt, task.UpdatedAt)
	return err
}

// Get implements persistence.TaskRepository.
func (r *TaskRepository) Get(ctx context.Context, id string) (*taskdsl.Task, error) {
	var t taskdsl.Task
	var planJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, prompt, state, plan_json, created_at, updated_at
		 FROM tasks WHERE id = $1`,
		id,
	).Scan(&t.ID, &t.Prompt, &t.State, &planJSON, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(planJSON) > 0 {
		if err := json.Unmarshal(planJSON, &t.Plan); err != nil {
			return nil, err
		}
	}
	return &t, nil
}

// Update implements persistence.TaskRepository.
func (r *TaskRepository) Update(ctx context.Context, task *taskdsl.Task) error {
	planJSON, err := json.Marshal(task.Plan)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE tasks SET prompt=$2, state=$3, plan_json=$4, updated_at=$5 WHERE id=$1`,
		task.ID, task.Prompt, task.State, planJSON, task.UpdatedAt)
	return err
}

// Close closes the connection pool.
func (r *TaskRepository) Close() error {
	r.pool.Close()
	return nil
}

var _ persistence.TaskRepository = (*TaskRepository)(nil)
