# 0005 — Reset-first schema-in-code

## Status

Accepted.

## Date

2026-05-18

## Context

Atlas Core applies Postgres shape in application startup code
(`datastorage/internal/postgres/schema.go`) under an advisory lock: idempotent
`CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, and
`ADD CONSTRAINT ... NOT VALID`. The project does not use SQL migration frameworks.

A service-posture review noted that this is already forward schema evolution and
that preserved-data upgrades would need operator playbooks (constraint validation,
version markers, release logging). Atlas Core is not targeting in-place database
upgrades: operators reset the stack and volumes when schema or binaries change.

## Decision

- **No migration framework.** Do not add versioned SQL migration files unless this
  ADR is superseded.
- **Startup DDL only.** Idempotent schema setup runs in `postgres.InitSchema` during
  datastorage startup (and tests). It must remain safe to re-run.
- **Reset, not upgrade.** When schema or service code changes, use a full reset of
  Postgres and object storage (for example `python3 atlas.py reset` or equivalent).
  Do not design for retaining Postgres rows across releases in the current product
  model.
- **No schema version ledger.** Do not add `atlas_schema_meta`, `CurrentSchemaVersion`,
  or similar application-level schema version tracking unless this ADR is superseded.
- **Constraint changes stay in code.** New checks may use `NOT VALID` in startup SQL
  when useful; with reset-first deployments, operators are not expected to run manual
  `VALIDATE CONSTRAINT` workflows for legacy dirty rows.

## Consequences

- Schema changes require editing `schema.go` and tests under
  `datastorage/internal/postgres`; there is no version bump ritual.
- Deployments that must keep existing Postgres data across binary upgrades are out of
  scope until the product explicitly chooses that model and supersedes this ADR.
- If preserved-data upgrades become a requirement later, add a new ADR that defines
  upgrade discipline; do not bolt on version tables without that decision.

## Supersedes

—

## Superseded by

—
