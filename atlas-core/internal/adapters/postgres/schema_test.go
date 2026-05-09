package postgres

import (
	"context"
	"testing"

	"github.com/anomalyco/atlas-core/internal/runtime/logging"
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
		`DROP TABLE IF EXISTS idempotency_keys`,
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
		`ALTER TABLE objects
			ADD CONSTRAINT objects_type_check
			CHECK (type IN ('command_catalog', 'log', 'photo'))`,
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
		"entities_type_check":           false,
		"idempotency_keys_status_check": false,
		"objects_type_check":            false,
		"objects_owner_type_check":      false,
		"tasks_status_check":            false,
	}

	rows, err := pool.Query(ctx, `
		SELECT conname
		FROM pg_constraint
		WHERE conname = ANY($1)
	`, []string{
		"entities_type_check",
		"idempotency_keys_status_check",
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

func TestInitSchema_DoesNotBlockLegacyInvalidRows(t *testing.T) {
	pool, cfg := openTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS tasks`,
		`DROP TABLE IF EXISTS observations`,
		`DROP TABLE IF EXISTS objects`,
		`DROP TABLE IF EXISTS entities`,
		`DROP TABLE IF EXISTS idempotency_keys`,
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
		`INSERT INTO entities (entity_id, type, json, created_at, updated_at)
		 VALUES ('legacy_entity', 'legacy_type', '{}'::jsonb, NOW(), NOW()),
		        ('asset_ok', 'asset', '{}'::jsonb, NOW(), NOW())`,
		`INSERT INTO objects (object_id, type, owner_type, owner_id, json, created_at, updated_at)
		 VALUES ('legacy_object', 'legacy_type', 'legacy_owner', 'legacy', '{}'::jsonb, NOW(), NOW()),
		        ('object_ok', 'log', 'system', 'system', '{}'::jsonb, NOW(), NOW())`,
		`INSERT INTO tasks (task_id, status, asset_id, command_catalog_object_id, json, created_at, updated_at)
		 VALUES ('legacy_task', 'legacy_status', 'asset_ok', 'object_ok', '{}'::jsonb, NOW(), NOW())`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	if err := InitSchema(ctx, pool, logging.New(cfg, "test")); err != nil {
		t.Fatalf("InitSchema failed with legacy invalid rows: %v", err)
	}

	for name, stmt := range map[string]string{
		"entity type": `INSERT INTO entities (entity_id, type, json, created_at, updated_at)
			VALUES ('entity_bad_new', 'legacy_type', '{}'::jsonb, NOW(), NOW())`,
		"idempotency status": `INSERT INTO idempotency_keys (scope, key, resource_id, status)
			VALUES ('object_create', 'idem_bad_new', 'obj_bad_new', 'legacy_status')`,
		"object type": `INSERT INTO objects (object_id, type, owner_type, owner_id, json, created_at, updated_at)
			VALUES ('object_bad_new', 'legacy_type', 'system', 'system', '{}'::jsonb, NOW(), NOW())`,
		"task status": `INSERT INTO tasks (task_id, status, asset_id, command_catalog_object_id, json, created_at, updated_at)
			VALUES ('task_bad_new', 'legacy_status', 'asset_ok', 'object_ok', '{}'::jsonb, NOW(), NOW())`,
	} {
		if _, err := pool.Exec(ctx, stmt); err == nil {
			t.Fatalf("expected new invalid %s row to be rejected", name)
		}
	}
}

func TestInitSchema_RepairsLegacyCommandCatalogObjectType(t *testing.T) {
	pool, cfg := openTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS tasks`,
		`DROP TABLE IF EXISTS observations`,
		`DROP TABLE IF EXISTS objects`,
		`DROP TABLE IF EXISTS entities`,
		`DROP TABLE IF EXISTS idempotency_keys`,
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
		`INSERT INTO objects (object_id, type, owner_type, owner_id, json, created_at, updated_at)
		 VALUES ('command_catalog', 'command_catalog', 'system', 'system', '{}'::jsonb, NOW(), NOW())`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	if err := InitSchema(ctx, pool, logging.New(cfg, "test")); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	var objectType string
	if err := pool.QueryRow(ctx, `SELECT type FROM objects WHERE object_id = 'command_catalog'`).Scan(&objectType); err != nil {
		t.Fatalf("read repaired command catalog type: %v", err)
	}
	if objectType != "document" {
		t.Fatalf("expected command_catalog object type to be repaired to document, got %q", objectType)
	}
}
