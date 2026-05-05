package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/logging"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS entities (
    entity_id   TEXT PRIMARY KEY,
    type        TEXT NOT NULL CONSTRAINT entities_type_check CHECK (type IN ('asset', 'track', 'geofeature')),
    subtype     TEXT,
    alias       TEXT,
    json        JSONB NOT NULL DEFAULT '{}'::jsonb,
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS objects (
    object_id   TEXT PRIMARY KEY,
    type        TEXT NOT NULL CONSTRAINT objects_type_check CHECK (type IN ('command_catalog', 'log', 'photo')),
    owner_type  TEXT NOT NULL CONSTRAINT objects_owner_type_check CHECK (owner_type IN ('entity', 'observation', 'task', 'system')),
    owner_id    TEXT NOT NULL,
    json        JSONB NOT NULL DEFAULT '{}'::jsonb,
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
    task_id                   TEXT PRIMARY KEY,
    status                    TEXT NOT NULL CONSTRAINT tasks_status_check CHECK (status IN ('pending', 'acknowledged', 'completed', 'failed')),
    asset_id                  TEXT NOT NULL REFERENCES entities(entity_id),
    command_catalog_object_id TEXT NOT NULL REFERENCES objects(object_id),
    json                      JSONB NOT NULL DEFAULT '{}'::jsonb,
    version                   INTEGER NOT NULL DEFAULT 1,
    created_at                TIMESTAMPTZ NOT NULL,
    updated_at                TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS observations (
    observation_id  TEXT PRIMARY KEY,
    source_asset_id TEXT NOT NULL REFERENCES entities(entity_id),
    json            JSONB NOT NULL DEFAULT '{}'::jsonb,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key         TEXT PRIMARY KEY,
    scope       TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS entities_type_idx ON entities(type);
CREATE INDEX IF NOT EXISTS entities_updated_at_idx ON entities(updated_at DESC, entity_id ASC);

CREATE INDEX IF NOT EXISTS objects_owner_idx ON objects(owner_type, owner_id);
CREATE INDEX IF NOT EXISTS objects_type_idx ON objects(type);
CREATE INDEX IF NOT EXISTS objects_updated_at_idx ON objects(updated_at DESC, object_id ASC);

CREATE INDEX IF NOT EXISTS tasks_asset_idx ON tasks(asset_id);
CREATE INDEX IF NOT EXISTS tasks_status_idx ON tasks(status);
CREATE INDEX IF NOT EXISTS tasks_asset_status_updated_idx ON tasks(asset_id, status, updated_at DESC, task_id ASC);
CREATE INDEX IF NOT EXISTS tasks_updated_at_idx ON tasks(updated_at DESC, task_id ASC);

CREATE INDEX IF NOT EXISTS observations_source_asset_idx ON observations(source_asset_id);
CREATE INDEX IF NOT EXISTS observations_updated_at_idx ON observations(updated_at DESC, observation_id ASC);

CREATE INDEX IF NOT EXISTS idempotency_keys_scope_resource_idx ON idempotency_keys(scope, resource_id);
`

const schemaConstraintUpgradeSQL = `
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'entities_type_check' AND conrelid = 'entities'::regclass
    ) THEN
        ALTER TABLE entities
            ADD CONSTRAINT entities_type_check
            CHECK (type IN ('asset', 'track', 'geofeature')) NOT VALID;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'objects_type_check' AND conrelid = 'objects'::regclass
    ) THEN
        ALTER TABLE objects
            ADD CONSTRAINT objects_type_check
            CHECK (type IN ('command_catalog', 'log', 'photo')) NOT VALID;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'objects_owner_type_check' AND conrelid = 'objects'::regclass
    ) THEN
        ALTER TABLE objects
            ADD CONSTRAINT objects_owner_type_check
            CHECK (owner_type IN ('entity', 'observation', 'task', 'system')) NOT VALID;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'tasks_status_check' AND conrelid = 'tasks'::regclass
    ) THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_status_check
            CHECK (status IN ('pending', 'acknowledged', 'completed', 'failed')) NOT VALID;
    END IF;
END $$;

ALTER TABLE entities     ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE objects      ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE tasks        ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE observations ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;
`

func InitSchema(ctx context.Context, pool *pgxpool.Pool, log *logging.Logger) error {
	log.InfoContext(ctx, "postgres_schema", "creating database schema")

	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		log.ErrorContext(ctx, "postgres_schema", "database schema creation failed", logging.ErrorField(err))
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := pool.Exec(ctx, schemaConstraintUpgradeSQL); err != nil {
		log.ErrorContext(ctx, "postgres_schema", "database schema constraint upgrade failed", logging.ErrorField(err))
		return fmt.Errorf("upgrade schema constraints: %w", err)
	}

	log.InfoContext(ctx, "postgres_schema", "schema initialized successfully")
	return nil
}
