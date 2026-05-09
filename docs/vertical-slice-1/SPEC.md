# Atlas Core Vertical Slice 1: Storage, Stores, and Function Foundation

## Purpose

Vertical Slice 1 is the internal foundation of Atlas Core.

This slice should not build the public API. It should not build data fusion. It should not build the UI, SDK, tasking behavior, observation reporting behavior, or server-sent events.

The goal is:

> Atlas Core has its core storage systems, database schema, local file/object storage, basic stores, basic function layer, config, logging, startup tooling, and tests in place.

Later slices will build real behavior on top of this foundation.

## Core layer model

Atlas Core should be built in layers:

```text
Database and file storage
Store layer
Function layer
API layer (later)
```

Vertical Slice 1 only includes the bottom three layers:

- Database and file storage
- Store layer
- Function layer

It does not include the API layer.

## Required technology choices

### Language

Atlas Core should be written in Go.

Go is the right fit because Atlas Core needs:

- simple compiled deployment
- good PostgreSQL support
- good filesystem support
- predictable performance
- clean Docker packaging
- strong concurrency support for later internal workers and streaming behavior

Python should be used for developer tooling, not for the Atlas Core service itself.

### Database

Atlas Core should use PostgreSQL.

PostgreSQL is the better fit because Atlas Core will eventually have:

- frequent writes
- multiple related tables
- JSON metadata fields
- concurrent readers and writers
- internal worker access
- stronger constraints
- better query behavior as the system grows

SQLite is simpler, but Atlas Core is not just a tiny embedded app. It is a local-first control-plane service with operational data, object metadata, observations, tasks, and later fusion behavior.

### Object/file storage

Atlas Core should use local filesystem storage through a Docker volume.

Do not use:

- MinIO
- S3
- distributed object storage
- a separate object storage service

Object files should live in a local Docker volume mounted into the Atlas Core container.

Example: `/var/lib/atlas-core/objects/`

#### ObjectID validation and filesystem-safety requirements

All ObjectID values must be strictly validated to prevent path traversal attacks and filesystem conflicts:

**Allowed characters:**
- Alphanumeric: `a-z`, `A-Z`, `0-9`
- Hyphen: `-`
- Underscore: `_`

**Forbidden patterns:**
- Path separators: `/`, `\`
- Parent directory segments: `..`
- Empty strings
- Reserved control filenames: `manifest.json`, `.`, `..`, and any system-reserved names

**Path construction rules:**
- All object paths MUST be constructed by joining the validated ObjectID to the base storage directory (e.g., `/var/lib/atlas-core/objects/`) using a safe path-join API
- After path construction, implementations MUST assert that the resulting absolute path starts with the storage root directory to prevent path traversal
- Implementations MUST NOT follow symlinks when accessing object files (use `O_NOFOLLOW` or equivalent file open flags)
- Implementations MUST validate that the final resolved inode is within the storage volume boundary

**Required tests:**
- Reject invalid ObjectIDs containing forbidden characters or patterns (e.g., `../etc/passwd`, `object/../other`, `/absolute/path`, `object/subdir`)
- Reject attempts to reference reserved filenames as ObjectIDs (e.g., `manifest.json`, `.`, `..`)
- Detect and reject symlink attacks (e.g., ObjectID that is a symlink pointing outside the storage root)
- Verify path traversal prevention (e.g., ensure constructed paths always remain within storage root)

### Database migrations

Atlas Core should not use database migrations during this development phase.

The system is still in active design. Preserving old local development data is not important right now.

Instead:

- schema is defined directly in the codebase
- startup creates the current schema
- stop/reset deletes the database volume
- restart recreates the database from the current schema

Schema changes are handled by changing the schema definition and rebuilding from scratch.

## Startup tool

Atlas Core should be managed through a Python script named `atlas.py`.

Running the script should show a terminal menu:

1. Start system
2. Stop / Reset system
3. Restart system

### Option 1: Start system

Start should:

- start the PostgreSQL container
- start the Atlas Core Go service container
- create required Docker volumes
- initialize the database schema
- initialize the object storage root
- show useful startup output
- fail fast on the first Docker/log/bootstrap error
- exit non-zero if startup does not complete successfully

### Option 2: Stop / Reset system

Stop means destructive reset. Stop should:

- stop Atlas Core containers
- remove Atlas Core containers
- remove the PostgreSQL data volume
- remove the object storage volume
- remove local runtime state
- remove orphan containers
- remove local images if the project chooses to include that in reset behavior
- fail fast on the first reset error
- exit non-zero if reset does not complete successfully

Stop does not mean "pause." Stop means: shut down and delete all local state.

### Option 3: Restart system

Restart should:

- run the full stop/reset behavior
- wait briefly
- start the system again from a clean state
- abort immediately if reset fails
- exit non-zero if either reset or start fails

Restart means: full reset plus clean start.

## Runtime foundation

Vertical Slice 1 should include:

- Go service entrypoint
- config loading
- environment validation
- structured logging
- PostgreSQL connection setup
- object storage root setup
- startup validation
- shutdown behavior
- test configuration
- Docker Compose support
- `atlas.py` control script

No HTTP server is required in this slice.

No public API routes should be implemented in this slice.

## Logging foundation

Logging should be included from the beginning.

The logging system should support:

- startup logs
- config load errors
- database connection errors
- schema setup errors
- object storage errors
- store operation errors
- function-level logs
- test/debug logs

Later slices can add:

- request IDs
- API request logs
- SSE logs
- internal worker logs
- audit logs

## Database schema

Vertical Slice 1 should create the base Atlas Core schema.

The core tables are:

- entities
- objects
- tasks
- observations
- idempotency_keys

There should be no `object_files` table in this slice.

### Table: entities

The entities table stores current-state records for things Atlas Core knows about.

Entity types: `asset`, `track`, `geofeature`

Columns:

- `entity_id` TEXT PRIMARY KEY
- `type` TEXT NOT NULL
- `subtype` TEXT
- `alias` TEXT
- `json` JSONB NOT NULL DEFAULT '{}'
- `version` INTEGER NOT NULL DEFAULT 1
- `created_at` TIMESTAMPTZ NOT NULL
- `updated_at` TIMESTAMPTZ NOT NULL

Basic indexes:

- `entities_type_idx` ON entities(type)
- `entities_updated_at_idx` ON entities(updated_at DESC, entity_id ASC)

### Table: objects

The objects table stores the database index for logical objects.

Columns:

- `object_id` TEXT PRIMARY KEY
- `type` TEXT NOT NULL
- `owner_type` TEXT NOT NULL
- `owner_id` TEXT NOT NULL
- `json` JSONB NOT NULL DEFAULT '{}'
- `version` INTEGER NOT NULL DEFAULT 1
- `created_at` TIMESTAMPTZ NOT NULL
- `updated_at` TIMESTAMPTZ NOT NULL

Supported owner types: `entity`, `observation`, `task`, `system`

Basic indexes:

- `objects_owner_idx` ON objects(owner_type, owner_id)
- `objects_type_idx` ON objects(type)
- `objects_updated_at_idx` ON objects(updated_at DESC, object_id ASC)

### Table: tasks

The tasks table stores work assigned to assets.

Columns:

- `task_id` TEXT PRIMARY KEY
- `status` TEXT NOT NULL
- `asset_id` TEXT NOT NULL REFERENCES entities(entity_id)
- `command_catalog_object_id` TEXT NOT NULL REFERENCES objects(object_id)
- `json` JSONB NOT NULL DEFAULT '{}'
- `version` INTEGER NOT NULL DEFAULT 1
- `created_at` TIMESTAMPTZ NOT NULL
- `updated_at` TIMESTAMPTZ NOT NULL

Task statuses: `pending`, `acknowledged`, `completed`, `failed`

Basic indexes:

- `tasks_asset_idx` ON tasks(asset_id)
- `tasks_status_idx` ON tasks(status)
- `tasks_asset_status_updated_idx` ON tasks(asset_id, status, updated_at DESC, task_id ASC)
- `tasks_updated_at_idx` ON tasks(updated_at DESC, task_id ASC)

### Table: observations

The observations table stores source-reported evidence.

Columns:

- `observation_id` TEXT PRIMARY KEY
- `source_asset_id` TEXT NOT NULL REFERENCES entities(entity_id)
- `json` JSONB NOT NULL DEFAULT '{}'
- `version` INTEGER NOT NULL DEFAULT 1
- `created_at` TIMESTAMPTZ NOT NULL
- `updated_at` TIMESTAMPTZ NOT NULL

Basic indexes:

- `observations_source_asset_idx` ON observations(source_asset_id)
- `observations_updated_at_idx` ON observations(updated_at DESC, observation_id ASC)

### Optimistic versioning

The `version` column on `entities`, `objects`, `tasks`, and `observations` is
for optimistic concurrency on current-state rows. It is not an audit log and it
is not the future changefeed sequence number.

- New rows start at `1`, so "present and never updated" is distinguishable from
  "missing" without introducing a zero sentinel.
- `Update*` store methods treat the caller-provided `model.*.Version` as a
  compare-and-swap precondition: the write succeeds only when the stored row
  still has that version.
- A successful `Update*` increments the row to `version + 1`.
- If the target row exists but the version is stale, stores return
  `ErrVersionConflict`; if the row is gone, they return `ErrNotFound`.
- `Create*` initializes `version` to `1`. `Upsert*` inserts with `1` and bumps
  the existing row on the update path.
- Function-layer code should read the current row before mutating it and pass
  the current version through to the store rather than guessing or merging.

`objects.version` applies to the object's main metadata row. Manifest cache
writes use `manifest.Version` inside the manifest payload instead, so
`UpdateObjectManifest` does not bump the row version.

### Table: idempotency_keys

The idempotency_keys table stores internal claims/results for create operations
that use `WithIdempotencyKey(...)`.

In Slice 1 this is an internal function-layer mechanism, not a public API
contract. `WithIdempotencyKey(...)` is the option used by
`ObjectFunctions.CreateObject` and `TaskFunctions.CreateTask` to ask the
internal `IdempotencyStore` to deduplicate retried creates. The key itself is
an opaque non-empty string scoped by operation, not a schema-managed UUID.

Columns:

- `key` TEXT NOT NULL
- `scope` TEXT NOT NULL
- `resource_id` TEXT NOT NULL
- `status` TEXT NOT NULL DEFAULT 'pending'
- `created_at` TIMESTAMPTZ NOT NULL DEFAULT NOW()
- `updated_at` TIMESTAMPTZ NOT NULL DEFAULT NOW()

Primary key:

- `(scope, key)`

Allowed statuses: `pending`, `completed`, `failed`

Current Slice 1 scopes are:

- `object_create`
- `task_create`

Lifecycle:

- `TryBegin(...)` inserts `pending` when a create first claims the key.
- Fresh success transitions the record to `completed`.
- Fresh failure transitions the record to `failed`.
- A retry may reclaim a `failed` record for the same `resource_id` and move it
  back through `pending`.
- A retry that finds `completed` is treated as an idempotent replay: the create
  returns success without creating a duplicate row or re-running future publish
  hooks.
- A retry that finds the same key already `pending` is treated as "work already
  in progress"; the create path does not claim a second fresh operation.

Replay/version interaction:

- A completed replay preserves the original resource identity recorded in
  `resource_id`.
- The create functions currently return only `error`, so they do not reload and
  return stored row/version metadata on replay.
- Fresh creates still initialize the resource row's `version` to `1`; replaying
  a completed create does not bump that version.

Retention:

- Slice 1 defines no automatic TTL or purge job for `idempotency_keys`.
- Rows are retained until future maintenance work introduces an explicit cleanup
  policy.

Basic indexes:

- `idempotency_keys_scope_resource_idx` ON idempotency_keys(scope, resource_id)

### Object storage folder model

Each object gets a folder in local object storage.

Example: `/var/lib/atlas-core/objects/{object_id}/`

The database stores the logical object record. The filesystem stores the actual bytes. The object manifest tracks the files inside the object folder.

### Object manifest persistence model

Because Vertical Slice 1 does not use an `object_files` table, the object manifest becomes the authoritative file index for that object.

**Storage location:**
- Manifests are stored in **both** the database (`objects.json` JSONB column) and the filesystem (`objects/{object_id}/manifest.json`)
- The database copy is stored under the nested JSON path `objects.json["manifest"]`, not by replacing the entire `objects.json` document

**Single source of truth:**
- The **filesystem** (`objects/{object_id}/manifest.json`) is the canonical source of truth for object manifests
- The database `objects.json` column may cache manifest data for query performance, but the filesystem manifest is authoritative

**Synchronization rules:**

When writing or updating a manifest:
1. Write the manifest to the filesystem first: `objects/{object_id}/manifest.json`
2. Then update the nested database cache key `objects.json["manifest"]` with the same manifest data
3. If the filesystem write succeeds but the database update fails, the operation MUST return an explicit error/result object indicating DB-sync failure (or schedule a mandatory reconciliation job) to prevent silent durable drift. Callers must be able to detect this partial-failure state, retry the operation, or alert on the inconsistency. Merely logging and continuing is not acceptable.
4. If the filesystem write fails, abort the operation and do not update the database

When reading a manifest via `GetObjectManifest`:
- Read from the filesystem (`objects/{object_id}/manifest.json`)
- The database copy under `objects.json["manifest"]` is used only as a cache for queries that need manifest metadata without filesystem access

**Drift resolution:**
- If drift is detected between the filesystem manifest and the database JSONB (e.g., during validation or repair operations), the filesystem version wins
- Implementations should provide a repair/reconciliation function that rebuilds the database `objects.json` column from the filesystem manifest
- Normal operations should maintain synchronization by following the write ordering above

**Error handling:**
- If a filesystem manifest exists but the database record is missing or corrupted, treat the filesystem as authoritative and rebuild the database record
- If a database record exists but the filesystem manifest is missing, treat this as a fatal inconsistency (the object is broken)

This model ensures that the filesystem manifest referenced in the "Object storage folder model" section above remains the definitive record, while allowing the database to cache manifest data for performance.

## Store layer

Stores should be simple — no workflow logic, no HTTP, no fusion, no SSE, no high-level behavior.

### Store list

- **EntityStore**: CreateEntity, GetEntity, ListEntities, UpdateEntity, DeleteEntity, UpsertEntity
- **ObjectStore**: CreateObject, GetObject, ListObjects, UpdateObject, DeleteObject, UpsertObject, UpdateObjectManifest, GetObjectManifest
- **TaskStore**: CreateTask, GetTask, ListTasks, UpdateTask, DeleteTask, UpsertTask
- **ObservationStore**: CreateObservation, GetObservation, ListObservations, UpdateObservation, DeleteObservation, UpsertObservation
- **ObjectStorageStore**: CreateObjectFolder, ObjectFolderExists, DeleteObjectFolder, WriteObjectFile, AppendObjectFile, ReadObjectFile, ReaderForObjectFile, DeleteObjectFile, ListObjectFolderFiles, ReadManifestFile, WriteManifestFile, ValidateSafeObjectPath

## Function layer

The function layer sits above the stores. In Vertical Slice 1, the function layer should mostly mirror basic store operations.

Functions should own:

- basic input validation
- store calls
- simple multi-step coordination
- logging
- error normalization
- future event emission points (planned hooks where Slice 2 changefeed/SSE work
  can publish domain changes after successful mutations; see
  `CHANGEFEED-HOOK.md`)

### Mutation boundary

All Atlas Core state mutations go through Atlas Core functions.

The API will eventually call functions. Stores should not be called directly by public API handlers later. Normal system behavior should not bypass the function layer.

## Testing scope

Tests should include:

- config loading
- logging setup
- database connection
- schema creation
- object storage root creation
- object folder creation
- manifest read/write
- safe object path validation
- EntityStore CRUD
- ObjectStore CRUD
- TaskStore CRUD
- ObservationStore CRUD
- ObjectStorageStore file operations
- basic entity functions
- basic object functions
- basic object file functions
- basic task functions
- basic observation functions
- function-to-store wiring
- error behavior
- cleanup behavior

## What Vertical Slice 1 intentionally does not solve

- public API endpoints
- object upload API
- asset observation reporting
- observation history spillover
- data fusion
- track creation
- task command validation
- task lifecycle transitions
- server-sent events
- SDK
- UI
- auth
- deployment scaling
- compression
- trajectory simplification
