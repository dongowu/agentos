package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dongowu/agentos/internal/adapter"
	"github.com/dongowu/agentos/internal/persistence"
	"github.com/dongowu/agentos/pkg/config"
	"github.com/dongowu/agentos/pkg/taskdsl"
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
	adapter.RegisterAuditLogStore("postgres", func(ctx context.Context, cfg config.PersistenceConfig) (persistence.AuditLogStore, error) {
		dsn := cfg.Postgres.DSN
		if dsn == "" {
			dsn = "postgres://agentos:agentos@localhost:5432/agentos?sslmode=disable"
		}
		return NewAuditLogStore(ctx, dsn)
	})
}

// TaskRepository is a PostgreSQL implementation of persistence.TaskRepository.
type TaskRepository struct {
	pool *pgxpool.Pool
}

// AuditLogStore is a PostgreSQL implementation of persistence.AuditLogStore.
type AuditLogStore struct {
	pool *pgxpool.Pool
}

// NewTaskRepository creates tables and returns a Postgres task repository.
func NewTaskRepository(ctx context.Context, dsn string) (*TaskRepository, error) {
	pool, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &TaskRepository{pool: pool}, nil
}

// NewAuditLogStore creates tables and returns a Postgres audit store.
func NewAuditLogStore(ctx context.Context, dsn string) (*AuditLogStore, error) {
	pool, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &AuditLogStore{pool: pool}, nil
}

func openPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
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
	return pool, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			prompt TEXT NOT NULL,
			tenant_id TEXT NOT NULL DEFAULT '',
			agent_name TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL,
			plan_json JSONB,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)
	`); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS agent_name TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS audit_logs (
			task_id TEXT NOT NULL,
			action_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL DEFAULT '',
			agent_name TEXT NOT NULL DEFAULT '',
			command TEXT NOT NULL,
			runtime_env TEXT NOT NULL DEFAULT '',
			worker_id TEXT NOT NULL DEFAULT '',
			exit_code INT NOT NULL,
			stdout TEXT NOT NULL DEFAULT '',
			stderr TEXT NOT NULL DEFAULT '',
			error_text TEXT NOT NULL DEFAULT '',
			side_effects JSONB NOT NULL DEFAULT '[]'::jsonb,
			occurred_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (task_id, action_id)
		)
	`)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS agent_name TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	return nil
}

// Create implements persistence.TaskRepository.
func (r *TaskRepository) Create(ctx context.Context, task *taskdsl.Task) error {
	planJSON, err := json.Marshal(task.Plan)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO tasks (id, prompt, tenant_id, agent_name, state, plan_json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		task.ID, task.Prompt, task.TenantID, task.AgentName, task.State, planJSON, task.CreatedAt, task.UpdatedAt)
	return err
}

// Get implements persistence.TaskRepository.
func (r *TaskRepository) Get(ctx context.Context, id string) (*taskdsl.Task, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, prompt, tenant_id, agent_name, state, plan_json, created_at, updated_at
		 FROM tasks WHERE id = $1`,
		id,
	)
	t, err := scanTask(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// Update implements persistence.TaskRepository.
func (r *TaskRepository) Update(ctx context.Context, task *taskdsl.Task) error {
	planJSON, err := json.Marshal(task.Plan)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE tasks SET prompt=$2, tenant_id=$3, agent_name=$4, state=$5, plan_json=$6, updated_at=$7 WHERE id=$1`,
		task.ID, task.Prompt, task.TenantID, task.AgentName, task.State, planJSON, task.UpdatedAt)
	return err
}

// ListRecoverable returns queued and running tasks in creation order.
func (r *TaskRepository) ListRecoverable(ctx context.Context) ([]*taskdsl.Task, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, prompt, tenant_id, agent_name, state, plan_json, created_at, updated_at
		FROM tasks
		WHERE state IN ('queued', 'running')
		ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*taskdsl.Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdsl.Task, error) {
	var task taskdsl.Task
	var planJSON []byte
	if err := scanner.Scan(&task.ID, &task.Prompt, &task.TenantID, &task.AgentName, &task.State, &planJSON, &task.CreatedAt, &task.UpdatedAt); err != nil {
		return nil, err
	}
	if len(planJSON) > 0 {
		if err := json.Unmarshal(planJSON, &task.Plan); err != nil {
			return nil, err
		}
	}
	return &task, nil
}

// Append implements persistence.AuditLogStore.
func (s *AuditLogStore) Append(ctx context.Context, record persistence.AuditRecord) error {
	sideEffectsJSON, err := json.Marshal(record.SideEffects)
	if err != nil {
		return err
	}
	occurredAt := record.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_logs (task_id, action_id, tenant_id, agent_name, command, runtime_env, worker_id, exit_code, stdout, stderr, error_text, side_effects, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (task_id, action_id)
		DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			agent_name = EXCLUDED.agent_name,
			command = EXCLUDED.command,
			runtime_env = EXCLUDED.runtime_env,
			worker_id = EXCLUDED.worker_id,
			exit_code = EXCLUDED.exit_code,
			stdout = EXCLUDED.stdout,
			stderr = EXCLUDED.stderr,
			error_text = EXCLUDED.error_text,
			side_effects = EXCLUDED.side_effects,
			occurred_at = EXCLUDED.occurred_at`,
		record.TaskID, record.ActionID, record.TenantID, record.AgentName, record.Command, record.RuntimeEnv, record.WorkerID, record.ExitCode,
		record.Stdout, record.Stderr, record.Error, sideEffectsJSON, occurredAt,
	)
	return err
}

// Get implements persistence.AuditLogStore.
func (s *AuditLogStore) Get(ctx context.Context, taskID, actionID string) (*persistence.AuditRecord, error) {
	var record persistence.AuditRecord
	var sideEffectsJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT task_id, action_id, tenant_id, agent_name, command, runtime_env, worker_id, exit_code, stdout, stderr, error_text, side_effects, occurred_at
		FROM audit_logs WHERE task_id = $1 AND action_id = $2`, taskID, actionID,
	).Scan(
		&record.TaskID,
		&record.ActionID,
		&record.TenantID,
		&record.AgentName,
		&record.Command,
		&record.RuntimeEnv,
		&record.WorkerID,
		&record.ExitCode,
		&record.Stdout,
		&record.Stderr,
		&record.Error,
		&sideEffectsJSON,
		&record.OccurredAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(sideEffectsJSON) > 0 {
		if err := json.Unmarshal(sideEffectsJSON, &record.SideEffects); err != nil {
			return nil, err
		}
	}
	return &record, nil
}

// ListByTask implements persistence.AuditLogStore.
func (s *AuditLogStore) ListByTask(ctx context.Context, taskID string) ([]persistence.AuditRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT task_id, action_id, tenant_id, agent_name, command, runtime_env, worker_id, exit_code, stdout, stderr, error_text, side_effects, occurred_at
		FROM audit_logs WHERE task_id = $1 ORDER BY occurred_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []persistence.AuditRecord
	for rows.Next() {
		var record persistence.AuditRecord
		var sideEffectsJSON []byte
		if err := rows.Scan(
			&record.TaskID,
			&record.ActionID,
			&record.TenantID,
			&record.AgentName,
			&record.Command,
			&record.RuntimeEnv,
			&record.WorkerID,
			&record.ExitCode,
			&record.Stdout,
			&record.Stderr,
			&record.Error,
			&sideEffectsJSON,
			&record.OccurredAt,
		); err != nil {
			return nil, err
		}
		if len(sideEffectsJSON) > 0 {
			if err := json.Unmarshal(sideEffectsJSON, &record.SideEffects); err != nil {
				return nil, err
			}
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// Query implements persistence.AuditLogStore.
func (s *AuditLogStore) Query(ctx context.Context, query persistence.AuditQuery) ([]persistence.AuditRecord, error) {
	where := []string{"1=1"}
	args := []any{}
	arg := func(value any) string {
		args = append(args, value)
		return "$" + fmt.Sprint(len(args))
	}
	if query.TaskID != "" {
		where = append(where, "task_id = "+arg(query.TaskID))
	}
	if query.ActionID != "" {
		where = append(where, "action_id = "+arg(query.ActionID))
	}
	if query.TenantID != "" {
		where = append(where, "tenant_id = "+arg(query.TenantID))
	}
	if query.AgentName != "" {
		where = append(where, "agent_name = "+arg(query.AgentName))
	}
	if query.WorkerID != "" {
		where = append(where, "worker_id = "+arg(query.WorkerID))
	}
	if query.FailedOnly {
		where = append(where, "(error_text <> '' OR exit_code <> 0)")
	}
	sql := `
		SELECT task_id, action_id, tenant_id, agent_name, command, runtime_env, worker_id, exit_code, stdout, stderr, error_text, side_effects, occurred_at
		FROM audit_logs WHERE ` + strings.Join(where, " AND ") + ` ORDER BY occurred_at DESC`
	if query.Limit > 0 {
		sql += " LIMIT " + arg(query.Limit)
	}
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []persistence.AuditRecord
	for rows.Next() {
		var record persistence.AuditRecord
		var sideEffectsJSON []byte
		if err := rows.Scan(&record.TaskID, &record.ActionID, &record.TenantID, &record.AgentName, &record.Command, &record.RuntimeEnv, &record.WorkerID, &record.ExitCode, &record.Stdout, &record.Stderr, &record.Error, &sideEffectsJSON, &record.OccurredAt); err != nil {
			return nil, err
		}
		if len(sideEffectsJSON) > 0 {
			if err := json.Unmarshal(sideEffectsJSON, &record.SideEffects); err != nil {
				return nil, err
			}
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// Close closes the connection pool.
func (r *TaskRepository) Close() error {
	r.pool.Close()
	return nil
}

// Close closes the connection pool.
func (s *AuditLogStore) Close() error {
	s.pool.Close()
	return nil
}

var _ persistence.TaskRepository = (*TaskRepository)(nil)
var _ persistence.AuditLogStore = (*AuditLogStore)(nil)
