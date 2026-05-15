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
- **Atlas Core architecture and quality notes**: `docs/atlas-core/design-decisions/` (ADRs) and the vertical slice specs under `docs/atlas-core/`.

This repository is not the legacy monolithic ATLAS tree (`Atlas_Command`, client SDKs, Meshtastic bridges, etc.). Do not assume paths or tooling from that repo unless they were intentionally mirrored here.

## Project notes and gotchas

- **Schema without migrations**: the project avoids migration frameworks; schema setup for Postgres lives in application code (`atlas-core/services/datastorage/internal/postgres`). Changing persistence shape means updating that code and any callers/tests—do not add SQL migration files to satisfy the same change.
- **Postgres-backed tests**: packages under `atlas-core/services/datastorage/internal/postgres` and integration-style tests in `atlas-core/services/functions/internal/function` expect a reachable Postgres instance. They use `ATLAS_TEST_POSTGRES_*` env vars when set; otherwise defaults target `localhost:5432` and database `atlas_core_test`. If the server is down, tests **skip** rather than fail hard (after connection attempt).
- **Test DB safety**: `atlas-core/services/datastorage/internal/postgres` test helpers refuse to run destructive cleanup unless the configured database name ends with `_test` or `ATLAS_ALLOW_DB_CLEANUP=true`. Do not point tests at a production database name.
- **Compose vs env files**: runtime env for Docker is wired in `atlas-core/docker-compose.yml`; local overrides are documented in `atlas-core/.env.example`. `ATLAS_POSTGRES_HOST_PORT` only affects host port binding for the Postgres service, not the in-network hostname the `atlas-core` service uses (`postgres`).
