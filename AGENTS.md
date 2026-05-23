# ATLAS2 Agent Guidance

## Agent notes purpose

The role of this file is to describe common mistakes and confusion points that agents might encounter while working in this project.

If you encounter anything in the project that surprises you, alert the developer you are working with and record that note in this `AGENTS.md` file so future agents can avoid the same issue.

## Core working rules

- Keep solutions simple and fully functional; avoid mockups.
- Do not use database migrations.
- Avoid mock or stub data in dev/prod code paths (tests only).
- Keep code modular and avoid duplicated logic.
- Avoid introducing new patterns/technologies when an existing implementation can solve the issue.
- Keep files reasonably small; refactor when files grow too large.
- Optimize for ease, clarity, and speed from the start, establishing patterns that are easy to contribute to, clear in function, and fast to change. Scope should be proportionate to impact: small changes should touch few files, while large changes may touch many.
- Tolerate nothing when it comes to bad patterns or code, but keep cleanup scoped unless broader changes are explicitly requested or required to unblock an immediate correctness or security issue.
- Use aggressive deletion or rebuilds only when explicitly requested by the reviewer or owner, or when a confirmed correctness or security blocker cannot be resolved surgically; otherwise, touch only what you must and clean up only your own mess.

## Behavioral guidelines

These guidelines are intended to reduce common LLM coding mistakes. Merge them with project-specific instructions as needed.
Tradeoff: These guidelines bias toward caution over speed. For trivial tasks, use judgment.

### 1. Think before coding

- State assumptions explicitly before implementing. If uncertain, ask.
- If multiple interpretations exist, present them instead of silently picking one.
- If a simpler approach exists, say so; push back when warranted. Avoid assumptions and confusion—surface tradeoffs and ask before proceeding if unclear.

### 2. Simplicity first

- Write the minimum code that solves the problem. Nothing speculative.
- Skip features beyond what was asked.
- Avoid abstractions for single-use code.
- Resist flexibility or configurability that was not requested.
- Avoid adding error-handling for well-established invariants, but prefer explicit assertions or lightweight logging/alerts for unexpected external I/O or validation failures that may become possible over time.
- If you write 200 lines and it could be 50, rewrite it.
- Ask: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical changes

- Surgical Changes is the default. Broader cleanup may take precedence only when explicitly requested by the reviewer or owner, or when required to resolve a confirmed correctness or security blocker that cannot be fixed surgically.
- Touch only what you must. Clean up only your own mess.
- Do not improve adjacent code, comments, or formatting unless the request requires it.
- Do not refactor things that are not broken.
- Match existing style, even if you would do it differently.
- Remove imports, variables, and functions that your own changes make unused; do not delete pre-existing or unrelated dead code unless explicitly requested.
- Every changed line should trace directly to the user's request.

### 4. Goal-driven execution

- Define success criteria that can be verified, then loop until verified.
- Translate vague tasks into checks:
  - "Add validation" -> write tests for invalid inputs, then make them pass.
  - "Fix the bug" -> write a test that reproduces it, then make it pass.
  - "Refactor X" -> ensure tests pass before and after.
- For multi-step tasks, state a brief plan with a verification step for each item.
- Strong success criteria enable independent execution. Weak criteria require clarification.

These guidelines are working if they produce fewer unnecessary diff changes, fewer rewrites due to overcomplication, and clarifying questions before implementation instead of after mistakes.

## Where to find things

- **Atlas Core services (Go)**: `atlas-core/` — service entrypoints live at `atlas-core/services/datastorage/cmd/atlas-datastorage/main.go` and `atlas-core/services/functions/cmd/atlas-functions/main.go`; Docker Compose and `Dockerfile` stay beside the module.
- **Shared service code and generated gRPC types**: `atlas-core/services/shared/`.
- **Local stack menu**: repo-root `atlas.py` runs codegen plus `docker compose` with working directory `atlas-core/` (start/stop/reset).
- **Atlas Core product/spec context**: `docs/atlas-core/vertical-slice-1/SPEC.md` and `docs/atlas-core/vertical-slice-2/SPEC.md`.
- **Atlas Protocol context**: `docs/atlas-protocol/README.md`, `docs/atlas-protocol/roadmap.md`, and `docs/atlas-protocol/contracts/README.md`.
- **Atlas Core design decisions (ADRs)**: `docs/atlas-core/design-decisions/` (see `README.md` for naming and purpose).
- **Short-lived problems log**: `problems/` — agent-to-agent notes on bugs and blockers (minutes to a couple of days); template in `problems/_EXAMPLE_PROBLEM_.md`. Recurring gotchas go in `AGENTS.md`; decisions in `docs/atlas-core/design-decisions/`; intended behavior in `docs/`.

This repository is not the legacy monolithic ATLAS tree (`Atlas_Command`, client SDKs, Meshtastic bridges, etc.). Do not assume paths or tooling from that repo unless they were intentionally mirrored here.

## Design decisions (authoritative)

Before changing service boundaries, compose exposure, tenancy, schema policy, or
HTTP idempotency, read the relevant ADR in `docs/atlas-core/design-decisions/`.
Do not restate or contradict those records in code comments or other docs—link
to them instead.

- `0001-api-boundary-idempotency-versioning.md` — HTTP idempotency and row version at the product edge
- `0002-service-boundaries-grpc-changefeed.md` — Service boundaries, gRPC entrypoints, changefeed
- `0003-internal-api-exposure-posture.md` — Compose reachability and exposure
- `0004-single-tenant-deployment-model.md` — Single-tenant deployment model
- `0005-reset-first-schema-in-code.md` — Reset-first schema-in-code

## Project notes and gotchas

- **Service boundaries and API entrypoints**: see `docs/atlas-core/design-decisions/0002-service-boundaries-grpc-changefeed.md`.
- **Compose exposure and reachability**: see `docs/atlas-core/design-decisions/0003-internal-api-exposure-posture.md`.
- **Single-tenant deployments**: see `docs/atlas-core/design-decisions/0004-single-tenant-deployment-model.md`.
- **Schema without migrations**: schema setup lives in `atlas-core/services/datastorage/internal/postgres`; see `docs/atlas-core/design-decisions/0005-reset-first-schema-in-code.md`.
- **Postgres-backed tests**: packages under `atlas-core/services/datastorage/internal/postgres` and Postgres-backed tests in `atlas-core/services/datastorage/internal/service` expect a reachable Postgres instance. They use `ATLAS_TEST_POSTGRES_*` env vars when set; otherwise defaults target `localhost:5432` and database `atlas_core_test`. If Postgres is unreachable, tests **fail** by default (via `testsupport.RequirePostgresOrSkip`). Set `ATLAS_SKIP_POSTGRES_TESTS=true` only for local runs without a database.
- **Test DB safety**: `atlas-core/services/datastorage/internal/postgres` test helpers refuse to run destructive cleanup unless the configured database name ends with `_test` or `ATLAS_ALLOW_DB_CLEANUP=true`. Do not point tests at a production database name.
- **Compose vs env files**: runtime env for Docker is wired in `atlas-core/docker-compose.yml`; local overrides are documented in `atlas-core/.env.example` (exposure rules: ADR 0003).
- **Codegen check behavior**: `python3 atlas.py codegen-check` is a git-cleanliness check for `atlas-core/services/shared/gen`, so it will fail after intentional proto edits until the regenerated files are committed. Use `python3 atlas.py codegen` to refresh the generated stubs before running the broader Go/build validation commands.
- **Protocol object types**: Atlas Core uses `command_catalog` (not `document`) for command catalogs. The protocol validator accepts both `command_catalog` and deprecated `document` in change-event snapshots; Core rejects `type: document` on create/update.
- **Observation history dedup index**: `historyContainsEventID` is read-only (no index writes during lookup). It checks (1) `extra.event_id_index` when `event_id_index_complete` is true, (2) `extra.event_id_index` when `event_id_index_complete` is false, (3) `event_ids.ndjson` in memory, then (4) a scan of `history.ndjson` without persisting. Call `bootstrapHistoryEventIDIndex` to rebuild/persist the index from sidecar or history. `appendHistoryEvent` treats `history.ndjson` as authoritative; sidecar/index maintenance failures are logged but do not fail the append. Ingest appends both `history.ndjson` and `event_ids.ndjson`. Do not grow `event_id_index` unbounded without a retention plan—it truncates and falls back to the sidecar.
