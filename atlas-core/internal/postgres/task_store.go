package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/store"
)

type TaskStore struct {
	pool *pgxpool.Pool
}

func NewTaskStore(pool *pgxpool.Pool) *TaskStore {
	return &TaskStore{pool: pool}
}

func (s *TaskStore) CreateTask(ctx context.Context, task *model.Task) error {
	jsonValue, err := jsonbParam(task.JSON)
	if err != nil {
		return fmt.Errorf("create task json: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO tasks (task_id, status, asset_id, command_catalog_object_id, json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)`,
		task.TaskID, task.Status, task.AssetID, task.CommandCatalogObjectID,
		jsonValue, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return model.ErrConflict
		}
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (s *TaskStore) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	task := &model.Task{}
	err := s.pool.QueryRow(ctx,
		`SELECT task_id, status, asset_id, command_catalog_object_id, json, created_at, updated_at
		 FROM tasks WHERE task_id = $1`, taskID,
	).Scan(
		&task.TaskID, &task.Status, &task.AssetID, &task.CommandCatalogObjectID,
		&task.JSON, &task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	return task, nil
}

func (s *TaskStore) ListTasks(ctx context.Context, filters ...store.TaskFilter) ([]model.Task, error) {
	state := &store.TaskFilterState{}
	for _, f := range filters {
		f(state)
	}

	query := `SELECT task_id, status, asset_id, command_catalog_object_id, json, created_at, updated_at FROM tasks`
	var conditions []string
	var args []interface{}
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
		args = append(args, *state.UpdatedAfter)
		argIdx++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY updated_at DESC, task_id ASC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.TaskID, &t.Status, &t.AssetID, &t.CommandCatalogObjectID,
			&t.JSON, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *TaskStore) UpdateTask(ctx context.Context, task *model.Task) error {
	jsonValue, err := jsonbParam(task.JSON)
	if err != nil {
		return fmt.Errorf("update task json: %w", err)
	}

	tag, err := s.pool.Exec(ctx,
		`UPDATE tasks SET status=$2, asset_id=$3, command_catalog_object_id=$4, json=$5::jsonb, updated_at=$6
		 WHERE task_id=$1`,
		task.TaskID, task.Status, task.AssetID, task.CommandCatalogObjectID,
		jsonValue, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (s *TaskStore) DeleteTask(ctx context.Context, taskID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM tasks WHERE task_id = $1`, taskID,
	)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (s *TaskStore) UpsertTask(ctx context.Context, task *model.Task) error {
	jsonValue, err := jsonbParam(task.JSON)
	if err != nil {
		return fmt.Errorf("upsert task json: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO tasks (task_id, status, asset_id, command_catalog_object_id, json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
		 ON CONFLICT (task_id) DO UPDATE SET
		   status=$2, asset_id=$3, command_catalog_object_id=$4, json=$5::jsonb, updated_at=$7`,
		task.TaskID, task.Status, task.AssetID, task.CommandCatalogObjectID,
		jsonValue, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert task: %w", err)
	}
	return nil
}
