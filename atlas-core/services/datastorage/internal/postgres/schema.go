package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/services/shared/logging"
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
    type        TEXT NOT NULL CONSTRAINT objects_type_check CHECK (type IN ('command_catalog', 'log', 'photo', 'observation_history', 'track_provenance')),
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
    observation_id       TEXT PRIMARY KEY,
    source_asset_id      TEXT NOT NULL REFERENCES entities(entity_id),
    target_entity_id     TEXT CONSTRAINT observations_target_entity_fkey REFERENCES entities(entity_id),
    started_at           TIMESTAMPTZ NOT NULL,
    ended_at             TIMESTAMPTZ,
    latest_telemetry_at  TIMESTAMPTZ,
    latest_identity_at   TIMESTAMPTZ,
    json                 JSONB NOT NULL DEFAULT '{}'::jsonb,
    version              INTEGER NOT NULL DEFAULT 1,
    created_at           TIMESTAMPTZ NOT NULL,
    updated_at           TIMESTAMPTZ NOT NULL,
    CONSTRAINT observations_ended_after_started CHECK (ended_at IS NULL OR ended_at >= started_at)
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key         TEXT NOT NULL,
    scope       TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scope, key)
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
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'objects_type_check' AND conrelid = 'objects'::regclass
    ) THEN
        ALTER TABLE objects DROP CONSTRAINT objects_type_check;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'objects_type_check' AND conrelid = 'objects'::regclass
    ) THEN
        ALTER TABLE objects
            ADD CONSTRAINT objects_type_check
            CHECK (type IN ('command_catalog', 'log', 'photo', 'observation_history', 'track_provenance')) NOT VALID;
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
ALTER TABLE observations ADD COLUMN IF NOT EXISTS target_entity_id TEXT;
ALTER TABLE observations ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ;
ALTER TABLE observations ADD COLUMN IF NOT EXISTS ended_at TIMESTAMPTZ;
ALTER TABLE observations ADD COLUMN IF NOT EXISTS latest_telemetry_at TIMESTAMPTZ;
ALTER TABLE observations ADD COLUMN IF NOT EXISTS latest_identity_at TIMESTAMPTZ;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'observations'
          AND column_name = 'observed_at'
    ) THEN
        UPDATE observations
        SET started_at = COALESCE(started_at, observed_at, created_at),
            latest_telemetry_at = COALESCE(latest_telemetry_at, observed_at)
        WHERE started_at IS NULL
           OR (latest_telemetry_at IS NULL AND observed_at IS NOT NULL);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'observations'
          AND column_name = 'started_at'
          AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE observations ALTER COLUMN started_at SET NOT NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'observations_target_entity_fkey' AND conrelid = 'observations'::regclass
    ) THEN
        ALTER TABLE observations
            ADD CONSTRAINT observations_target_entity_fkey
            FOREIGN KEY (target_entity_id) REFERENCES entities(entity_id) NOT VALID;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'observations_ended_after_started' AND conrelid = 'observations'::regclass
    ) THEN
        ALTER TABLE observations
            ADD CONSTRAINT observations_ended_after_started
            CHECK (ended_at IS NULL OR ended_at >= started_at) NOT VALID;
    END IF;
END $$;

ALTER TABLE idempotency_keys ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE idempotency_keys ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS observations_target_entity_idx ON observations(target_entity_id);
CREATE INDEX IF NOT EXISTS observations_started_at_idx ON observations(started_at DESC, observation_id ASC);
CREATE INDEX IF NOT EXISTS observations_latest_telemetry_at_idx ON observations(latest_telemetry_at DESC, observation_id ASC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'idempotency_keys_status_check' AND conrelid = 'idempotency_keys'::regclass
    ) THEN
        ALTER TABLE idempotency_keys
            ADD CONSTRAINT idempotency_keys_status_check
            CHECK (status IN ('pending', 'completed', 'failed')) NOT VALID;
    END IF;
END $$;
`

// schemaLockID is a well-known advisory lock id used to serialize concurrent
// schema creation across test packages that share the same database.
const schemaLockID = 1240826120575710875

func InitSchema(ctx context.Context, pool *pgxpool.Pool, log *logging.Logger) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.ErrorContext(ctx, "postgres_schema", "failed to acquire connection", logging.ErrorField(err))
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	log.InfoContext(ctx, "postgres_schema", "acquiring schema lock")

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, schemaLockID); err != nil {
		log.ErrorContext(ctx, "postgres_schema", "failed to acquire schema lock", logging.ErrorField(err))
		return fmt.Errorf("acquire schema lock: %w", err)
	}
	defer func() {
		var unlocked bool
		if err := conn.QueryRow(context.Background(), `SELECT pg_advisory_unlock($1)`, schemaLockID).Scan(&unlocked); err != nil {
			log.ErrorContext(ctx, "postgres_schema", "failed to release schema lock", logging.ErrorField(err))
		} else if !unlocked {
			log.ErrorContext(ctx, "postgres_schema", "failed to release schema lock", logging.String("error", "pg_advisory_unlock returned false"))
		}
	}()

	log.InfoContext(ctx, "postgres_schema", "creating database schema")

	if _, err := conn.Exec(ctx, schemaSQL); err != nil {
		log.ErrorContext(ctx, "postgres_schema", "database schema creation failed", logging.ErrorField(err))
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := conn.Exec(ctx, schemaConstraintUpgradeSQL); err != nil {
		log.ErrorContext(ctx, "postgres_schema", "database schema constraint upgrade failed", logging.ErrorField(err))
		return fmt.Errorf("upgrade schema constraints: %w", err)
	}

	log.InfoContext(ctx, "postgres_schema", "schema initialized successfully")
	return nil
}
