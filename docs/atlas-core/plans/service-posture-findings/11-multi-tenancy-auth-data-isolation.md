# Multi-Tenancy, Auth, and Data Isolation Hooks

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real future product risk, not a current vertical-slice bug.

## Evidence

- `atlas-core/services/datastorage/internal/postgres/schema.go:12` through
  `atlas-core/services/datastorage/internal/postgres/schema.go:63` define global
  primary keys for entities, objects, tasks, observations, and idempotency keys
  without tenant/workspace/project scope.
- `atlas-core/services/shared/model/types.go:46` through
  `atlas-core/services/shared/model/types.go:86` define model structs with no
  tenant/workspace/project field.
- `atlas-core/proto/atlas/shared/v1/common.proto` has resource request messages
  such as `ObjectRequest` and `TaskRequest` with no tenant/workspace/project
  scope.
- A repository search for `tenant`, `workspace`, and `project` under
  `atlas-core/proto`, `atlas-core/services/datastorage/internal/postgres`,
  `atlas-core/services/functions/internal`, and `atlas-core/services/shared/model`
  returned no model or schema scoping hooks.

## Reasoning

Single-tenancy is a reasonable vertical-slice default. The important part is to
make it intentional. Idempotency keys, changefeed streams, object ownership, and
authorization checks will all need a scope dimension if the product becomes
multi-tenant. Retrofitting that after external usage is expensive because it
changes primary keys, unique constraints, RPC request shape, and auth semantics.

## Best Fix

Before external multi-tenant usage, decide the isolation unit name and contract
(`tenant_id`, `workspace_id`, or similar). Then add it consistently to:

- Auth identity and authorization checks in functions.
- Database primary/unique keys and foreign keys.
- Idempotency key scope.
- Changefeed subscription filtering and event metadata.
- Public request messages or server-derived context.

Do not add a placeholder column now unless there is a concrete product decision;
document the current single-tenant assumption instead.
