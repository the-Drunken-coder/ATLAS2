# Schema-In-Code Release Discipline

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real operational risk once data preservation matters. Acceptable for the current
project rule of no migration framework, but it needs a release playbook.

## Evidence

- `atlas-core/services/datastorage/internal/postgres/schema.go:12` through
  `atlas-core/services/datastorage/internal/postgres/schema.go:81` create tables
  and indexes in application startup code.
- `atlas-core/services/datastorage/internal/postgres/schema.go:83` through
  `atlas-core/services/datastorage/internal/postgres/schema.go:150` perform
  forward schema adjustments with `ALTER TABLE`, `ADD COLUMN IF NOT EXISTS`, and
  `ADD CONSTRAINT ... NOT VALID`.
- `atlas-core/services/datastorage/internal/postgres/schema.go:156` through
  `atlas-core/services/datastorage/internal/postgres/schema.go:193` run schema
  setup during `InitSchema` under an advisory lock.

## Reasoning

This is already schema evolution. The advisory lock handles concurrent startup,
but it does not provide release history, validation status, rollback notes, or
operator visibility into partially applied DDL. `NOT VALID` is the right
low-disruption Postgres tool for adding constraints, but it creates a second
operational step: existing rows are not guaranteed clean until validation runs.

## Best Fix

Keep the no-migration-framework rule, but add a lightweight schema release
contract:

- A schema/version marker table maintained by application code.
- A documented rule for when startup DDL may run automatically.
- A documented rule for when `VALIDATE CONSTRAINT` is required and how to handle
  dirty existing rows.
- A local-dev rule that reset volumes are supported until the release playbook
  exists.

Avoid adding SQL migration files unless the project rule changes.
