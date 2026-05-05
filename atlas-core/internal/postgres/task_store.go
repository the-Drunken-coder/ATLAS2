package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/store"
)

type TaskStore struct {
	pool *pgxpool.Pool
	log  *logging.Logger
}

func NewTaskStore(pool *pgxpool.Pool, logs ...*logging.Logger) *TaskStore {
	return &TaskStore{pool: pool, log: loggerOrNop(logs...)}
}

func (s *TaskStore) CreateTask(ctx context.Context, task *model.Task) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	jsonValue, err := jsonbParam(task.JSON)
	if err != nil {
		return fmt.Errorf("create task json: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO tasks (task_id, status, asset_id, command_catalog_object_id, json, version, created_at, updated_at)
 VALUES ($1, $2, $3, $4, $5::jsonb, 1, $6, $7)`,
		task.TaskID, task.Status, task.AssetID, task.CommandCatalogObjectID, jsonValue, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return model.ErrConflict
		}
		s.log.ErrorContext(ctx, "postgres_task_store", "create task failed", logging.String("task_id", task.TaskID), logging.ErrorField(err))
		return fmt.Errorf("create task: %w", err)
	}
	task.Version = 1
	return nil
}

func (s *TaskStore) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	task := &model.Task{}
	err := s.pool.QueryRow(ctx,
		`SELECT task_id, status, asset_id, command_catalog_object_id, json, version, created_at, updated_at
 FROM tasks WHERE task_id = $1`, taskID,
	).Scan(&task.TaskID, &task.Status, &task.AssetID, &task.CommandCatalogObjectID, &task.JSON, &task.Version, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		s.log.ErrorContext(ctx, "postgres_task_store", "get task failed", logging.String("task_id", taskID), logging.ErrorField(err))
		return nil, fmt.Errorf("get task: %w", err)
	}
	return task, nil
}

func (s *TaskStore) ListTasks(ctx context.Context, filters ...store.TaskFilter) ([]model.Task, error) {
	state := &store.TaskFilterState{}
	for _, f := range filters {
		f(state)
	}

	query := `SELECT task_id, status, asset_id, command_catalog_object_id, json, version, created_at, updated_at FROM tasks`
	var conditions []string
	args := make([]any, 0, 3)
	argIdx := 1

	if state.AssetID != nil {
		conditions = append(conditions, fmt.Sprintf("asset_id = $%d", argIdx))
		args = append(args, *state.AssetID)
		argIdx++
	}
	if state.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *state.Status)
		argIdx++
	}
	if state.UpdatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("updated_at > $%d", argIdx))
		args = append(args, state.UpdatedAfter.UTC())
		argIdx++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY updated_at DESC, task_id ASC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_task_store", "list tasks failed", logging.ErrorField(err))
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.TaskID, &t.Status, &t.AssetID, &t.CommandCatalogObjectID, &t.JSON, &t.Version, &t.CreatedAt, &t.UpdatedAt); err != nil {
			s.log.ErrorContext(ctx, "postgres_task_store", "scan task failed", logging.ErrorField(err))
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// UpdateTask performs an optimistic-concurrency update. The caller must supply
// task.Version from the prior read; the update succeeds only when the row's
// version still matches.
func (s *TaskStore) UpdateTask(ctx context.Context, task *model.Task) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	jsonValue, err := jsonbParam(task.JSON)
	if err != nil {
		return fmt.Errorf("update task json: %w", err)
	}

	// Atomic CTE: attempt the update and classify the miss without a second round-trip.
	var newVersion sql.NullInt64
	var classification string
	err = s.pool.QueryRow(ctx,
		`WITH attempt AS (
		   UPDATE tasks SET status=$2, asset_id=$3, command_catalog_object_id=$4, json=$5::jsonb,
		     version = version + 1, updated_at=$6
		   WHERE task_id=$1 AND version=$7
		   RETURNING version
		 ),
		 check AS (
		   SELECT
		     CASE
		       WHEN EXISTS(SELECT 1 FROM attempt) THEN 'updated'
		       WHEN EXISTS(SELECT 1 FROM tasks WHERE task_id=$1) THEN 'conflict'
		       ELSE 'not_found'
		     END AS result,
		     (SELECT version FROM attempt LIMIT 1) AS ver
		 )
		 SELECT result, ver FROM check`,
		task.TaskID, task.Status, task.AssetID, task.CommandCatalogObjectID, jsonValue, task.UpdatedAt, task.Version,
	).Scan(&classification, &newVersion)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_task_store", "update task failed", logging.String("task_id", task.TaskID), logging.ErrorField(err))
		return fmt.Errorf("update task: %w", err)
	}
	switch classification {
	case "updated":
		if !newVersion.Valid {
			return fmt.Errorf("updated task missing new version")
		}
		task.Version = int(newVersion.Int64)
		return nil
	case "conflict":
		return model.ErrVersionConflict
	case "not_found":
		return model.ErrNotFound
	default:
		return fmt.Errorf("unexpected classification: %s", classification)
	}
}

func (s *TaskStore) DeleteTask(ctx context.Context, taskID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tasks WHERE task_id = $1`, taskID)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_task_store", "delete task failed", logging.String("task_id", taskID), logging.ErrorField(err))
		return fmt.Errorf("delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

// UpsertTask is the explicit-clobber escape hatch. It increments the version
// unconditionally on update and does not enforce the caller-supplied version.
func (s *TaskStore) UpsertTask(ctx context.Context, task *model.Task) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	jsonValue, err := jsonbParam(task.JSON)
	if err != nil {
		return fmt.Errorf("upsert task json: %w", err)
	}

	var newVersion int
	err = s.pool.QueryRow(ctx,
		`INSERT INTO tasks (task_id, status, asset_id, command_catalog_object_id, json, version, created_at, updated_at)
 VALUES ($1, $2, $3, $4, $5::jsonb, 1, $6, $7)
 ON CONFLICT (task_id) DO UPDATE SET
   status=$2, asset_id=$3, command_catalog_object_id=$4, json=$5::jsonb,
   version = tasks.version + 1, updated_at=$7
 RETURNING version`,
		task.TaskID, task.Status, task.AssetID, task.CommandCatalogObjectID, jsonValue, task.CreatedAt, task.UpdatedAt,
	).Scan(&newVersion)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_task_store", "upsert task failed", logging.String("task_id", task.TaskID), logging.ErrorField(err))
		return fmt.Errorf("upsert task: %w", err)
	}
	task.Version = newVersion
	return nil
}
