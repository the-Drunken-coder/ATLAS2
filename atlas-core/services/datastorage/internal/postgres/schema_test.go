package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/logging"
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

	if err := InitSchema(ctx, pool, logging.New(cfg.LogLevel, "atlas-test", "test")); err != nil {
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

	if err := InitSchema(ctx, pool, logging.New(cfg.LogLevel, "atlas-test", "test")); err != nil {
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

func TestInitSchema_BackfillsStartedAtFromObservedAt(t *testing.T) {
	pool, cfg := openTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	observedAt := time.Date(2026, 1, 15, 12, 30, 0, 0, time.UTC)

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
		`CREATE TABLE observations (
			observation_id TEXT PRIMARY KEY,
			source_asset_id TEXT NOT NULL REFERENCES entities(entity_id),
			target_entity_id TEXT,
			observed_at TIMESTAMPTZ,
			json JSONB NOT NULL DEFAULT '{}'::jsonb,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`INSERT INTO entities (entity_id, type, json, created_at, updated_at)
		 VALUES ('asset_legacy', 'asset', '{}'::jsonb, NOW(), NOW())`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO observations (
			observation_id, source_asset_id, observed_at, json, created_at, updated_at
		) VALUES (
			'obs_legacy', 'asset_legacy', $1, '{}'::jsonb, NOW(), NOW()
		)`, observedAt); err != nil {
		t.Fatalf("insert legacy observation: %v", err)
	}

	if err := InitSchema(ctx, pool, logging.New(cfg.LogLevel, "atlas-test", "test")); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	var startedAt time.Time
	var nullable string
	if err := pool.QueryRow(ctx, `
		SELECT started_at,
		       (SELECT is_nullable
		        FROM information_schema.columns
		        WHERE table_schema = current_schema()
		          AND table_name = 'observations'
		          AND column_name = 'started_at')
		FROM observations
		WHERE observation_id = 'obs_legacy'
	`).Scan(&startedAt, &nullable); err != nil {
		t.Fatalf("query observation: %v", err)
	}

	if !startedAt.Equal(observedAt) {
		t.Fatalf("started_at = %s, want %s", startedAt, observedAt)
	}
	if nullable != "NO" {
		t.Fatalf("started_at is_nullable = %q, want NO", nullable)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO observations (
			observation_id, source_asset_id, started_at, json, created_at, updated_at
		) VALUES (
			'obs_missing_started_at', 'asset_legacy', NULL, '{}'::jsonb, NOW(), NOW()
		)`); err == nil {
		t.Fatal("expected insert with NULL started_at to be rejected")
	}
}

func TestInitSchema_BackfillsStartedAtWhenObservedAtNull(t *testing.T) {
	pool, cfg := openTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	createdAt := time.Date(2026, 2, 1, 8, 0, 0, 0, time.UTC)

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
		`CREATE TABLE observations (
			observation_id TEXT PRIMARY KEY,
			source_asset_id TEXT NOT NULL REFERENCES entities(entity_id),
			target_entity_id TEXT,
			observed_at TIMESTAMPTZ,
			json JSONB NOT NULL DEFAULT '{}'::jsonb,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`INSERT INTO entities (entity_id, type, json, created_at, updated_at)
		 VALUES ('asset_legacy', 'asset', '{}'::jsonb, NOW(), NOW())`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO observations (
			observation_id, source_asset_id, observed_at, json, created_at, updated_at
		) VALUES (
			'obs_state_only', 'asset_legacy', NULL, '{"state":"active"}'::jsonb, $1, $1
		)`, createdAt); err != nil {
		t.Fatalf("insert legacy observation: %v", err)
	}

	if err := InitSchema(ctx, pool, logging.New(cfg.LogLevel, "atlas-test", "test")); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	var startedAt time.Time
	var nullable string
	if err := pool.QueryRow(ctx, `
		SELECT started_at,
		       (SELECT is_nullable
		        FROM information_schema.columns
		        WHERE table_schema = current_schema()
		          AND table_name = 'observations'
		          AND column_name = 'started_at')
		FROM observations
		WHERE observation_id = 'obs_state_only'
	`).Scan(&startedAt, &nullable); err != nil {
		t.Fatalf("query observation: %v", err)
	}

	if !startedAt.Equal(createdAt) {
		t.Fatalf("started_at = %s, want %s", startedAt, createdAt)
	}
	if nullable != "NO" {
		t.Fatalf("started_at is_nullable = %q, want NO", nullable)
	}
}
