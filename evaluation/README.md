# ATLAS2 Codebase Evaluation

All issues from the original evaluation are resolved on this branch, which is
stacked on `copilot/fix-issues-in-evaluation-folder` (PR #3).

## What landed in this round

### 01 — Cross-layer atomicity
- `Funcs.Object.Reconcile` repairs DB↔FS drift (orphan folder cleanup, missing
  folder recreation, manifest cache sync). Runs at startup and on a
  configurable interval (`ATLAS_RECONCILE_INTERVAL`).
- Manifest version stamp (SHA-256) lets drift be detected cheaply.
- Idempotency-key support: function-layer Create paths accept
  `WithIdempotencyKey(...)`. A new `idempotency_keys` table records claims and
  returns success on retry of the same key+resource, conflict on key reuse with
  a different resource. `function.WithIdempotencyKey`, `postgres.IdempotencyStore`,
  `store.IdempotencyStore`. Integration test:
  `TestObjectFunctions_CreateObjectIdempotencyKey`.

### 03 — Path traversal hardening
- New `safeOpenAt` per-component walk with `O_NOFOLLOW` at every level and a
  filesystem-device check at every step (`fstat` on each FD vs the storage
  root). TOCTOU window between Lstat and open is closed because the walk uses
  FD-rooted `unix.Openat` calls.
- `Store` now holds an open FD on the storage root from `InitRoot`; reads,
  writes, appends, listings, deletes, and manifest temp+rename all route
  through `safeOpenAt` / `safeMkdirAt` / `safeUnlinkAt` / `safeRenameAt`.
- `Store.Close()` releases the FD on shutdown.
- New test `TestSafeOpenAt_RejectsIntermediateSymlink` covers the case the
  original eval flagged as missing — a symlink at an intermediate path
  component (not the leaf). Plus the existing leaf cases still pass.
- New dependency: `golang.org/x/sys`.

### 05 — Error model
- `entity_store_test.go` migrated from `==`/`!=` to `errors.Is` (6 sites).
- Rollback paths in `function.go` use `errors.Join` to preserve the chain.
- `.golangci.yml` enables `errorlint` to prevent regressions.

### 08 — Concurrency
- `version INTEGER NOT NULL DEFAULT 1` column on entities, objects, tasks,
  observations. This branch added transitional schema-upgrade SQL (`ALTER TABLE
  … ADD COLUMN IF NOT EXISTS`) for existing databases, but the authoritative
  project policy for current development remains the reset-and-recreate
  approach in `docs/vertical-slice-1/SPEC.md`; do not introduce new migration
  flows unless that policy changes.
- `model.X.Version` field on every primary type.
- Optimistic concurrency on every `UpdateX`: `WHERE id = $1 AND version = $N`,
  increments on success, returns `model.ErrVersionConflict` when the row
  exists but the version moved on, `model.ErrNotFound` when the row is gone.
- `UpsertX` is the explicit-clobber escape hatch (increments unconditionally).
- `model.ErrVersionConflict` added.
- Tests: `TestEntityStore_UpdateAdvancesVersion`,
  `TestEntityStore_UpdateRejectsStaleVersion`.

### 09 — Validation
- Redundant `ValidateSafeObjectPath` calls removed from internal helpers
  (`objectPath`, `filePath`, `manifestPath`).
- Public-boundary validation added to `ListObjectFolderFiles` and
  `ReadManifestFile`.
- Comment block on the helpers documents that they assume validated input.

### 18 — Smaller issues
- Closed by 05: the smaller contract/status mismatches were handled as part of
  the error model cleanup, so they now share the same validation and typed error
  response behavior instead of being tracked as separate one-off fixes.
