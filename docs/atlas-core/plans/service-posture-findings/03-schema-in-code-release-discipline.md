# Schema-In-Code Release Discipline

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real operational risk once data preservation matters. It is acceptable under the
current project rule of no migration framework and the resettable-development
assumption, but the current code is already doing schema evolution and will need
a release playbook before persistent environments matter.

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

Keep the no-migration-framework rule. Do not add SQL migration files unless the
project rule changes. When the project starts preserving existing data across
releases, add a lightweight schema release contract:

- A schema/version marker table maintained by application code.
- A documented rule for when startup DDL may run automatically.
- A documented rule for when `VALIDATE CONSTRAINT` is required and how to handle
  dirty existing rows.
- Operator-visible logging or status for partially applied schema setup.
- A local-dev rule that reset volumes are supported while the release playbook is
  not yet in force.

Until then, the current reset-and-rebuild development model is consistent with
Vertical Slice 1.
