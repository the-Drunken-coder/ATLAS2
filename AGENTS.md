# ATLAS2 — agent overrides

This repo is **not** legacy ATLAS (`Atlas_Command`, Meshtastic bridges, old client SDK paths). Do not import paths or patterns from that tree unless they exist here.

**Surprises:** If something in this repo contradicts your expectations, tell the developer and add a short note to this file (or `problems/` for time-boxed blockers).

## Hard constraints

- **No database migrations.** Schema lives in code: `atlas-core/services/datastorage/internal/postgres` ([ADR 0005](docs/atlas-core/design-decisions/0005-reset-first-schema-in-code.md)).
- **Before changing** service boundaries, compose exposure, tenancy, HTTP idempotency, or row-version policy: read the matching ADR in `docs/atlas-core/design-decisions/` (0001–0005). Link ADRs; do not restate them in comments.

## Commands (use these; do not invent npm/docker wrappers)

- Stack / codegen: `python3 atlas.py` (`codegen`, `codegen-check`, `protocol-check`, `architecture-check`; compose cwd is `atlas-core/`).
- After `.proto` edits: `python3 atlas.py codegen`, then commit `atlas-core/services/shared/gen`.
- **protoc v34.1 only** (`libprotoc 34.1`; generated headers may say `protoc v7.34.1`). Do not use apt `protobuf-compiler` 3.x or regenerate stubs just to change header strings.

## Postgres tests

- Packages under `atlas-core/services/datastorage/internal/postgres` and Postgres-backed tests in `.../internal/service` need Postgres (`ATLAS_TEST_POSTGRES_*` or defaults: `localhost:5432`, DB `atlas_core_test`).
- Unreachable Postgres → tests **fail** (`RequirePostgresOrSkip` fails by default). `ATLAS_SKIP_POSTGRES_TESTS=true` only for local convenience, not CI regressions.
- Destructive test cleanup only if DB name ends with `_test` or `ATLAS_ALLOW_DB_CLEANUP=true`.

## Types agents get wrong

- Core object type for catalogs: **`command_catalog`**. Reject `type: document` on create/update. Protocol change-events may still mention `document` in snapshots; do not reintroduce `document` in Core writes.

## Observation history (`atlas-functions`)

Before changing dedup/index/append behavior, read `atlas-core/services/functions/internal/function/observation_history_dedup.go`.

- Lookup `historyContainsEventID` is **read-only** (no index writes on read).
- Append: `history.ndjson` is authoritative; index/sidecar failures are logged, not append failures.
- Dedup is **in-process per `history_object_id`** ([ADR 0004](docs/atlas-core/design-decisions/0004-single-tenant-deployment-model.md)) — do not add cross-replica dedup in datastorage.

## Scope discipline

- Surgical diffs only unless the user asks for broader cleanup or a security/correctness fix cannot be done narrowly.
- No mock/stub data outside tests.

## Services map (minimal)

- **datastorage** / **functions**: gRPC product path — `atlas-core/services/{datastorage,functions}/cmd/`.
- **fusion**: `atlas-fusion` in compose; not the same boundary as datastorage/functions — see `atlas-core/docker-compose.yml`.
