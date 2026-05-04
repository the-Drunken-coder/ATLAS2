package postgres

import (
	"context"
	"testing"

	"github.com/anomalyco/atlas-core/internal/logging"
)

func TestInitSchema_AddsConstraintsToExistingTables(t *testing.T) {
	pool, cfg := openTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS tasks`,
		`DROP TABLE IF EXISTS observations`,
		`DROP TABLE IF EXISTS objects`,
		`DROP TABLE IF EXISTS entities`,
		`CREATE TABLE entities (
			entity_id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			subtype TEXT,
			alias TEXT,
			json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE objects (
			object_id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			owner_type TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE tasks (
			task_id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			asset_id TEXT NOT NULL REFERENCES entities(entity_id),
			command_catalog_object_id TEXT NOT NULL REFERENCES objects(object_id),
			json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE observations (
			observation_id TEXT PRIMARY KEY,
			source_asset_id TEXT NOT NULL REFERENCES entities(entity_id),
			json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	if err := InitSchema(ctx, pool, logging.New(cfg, "test")); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	constraints := map[string]bool{
		"entities_type_check":      false,
		"objects_type_check":       false,
		"objects_owner_type_check": false,
		"tasks_status_check":       false,
	}

	rows, err := pool.Query(ctx, `
		SELECT conname
		FROM pg_constraint
		WHERE conname = ANY($1)
	`, []string{
		"entities_type_check",
		"objects_type_check",
		"objects_owner_type_check",
		"tasks_status_check",
	})
	if err != nil {
		t.Fatalf("query constraints: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan constraint: %v", err)
		}
		constraints[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate constraints: %v", err)
	}

	for name, found := range constraints {
		if !found {
			t.Fatalf("expected constraint %s to be present after InitSchema", name)
		}
	}
}
