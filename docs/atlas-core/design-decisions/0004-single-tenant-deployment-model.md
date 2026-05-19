# 0004 — Single-tenant deployment model

## Status

Accepted.

## Date

2026-05-18

## Context

Atlas Core schema, models, and RPC request messages use global resource identifiers
with no `tenant_id`, `workspace_id`, or similar scope dimension. That matches
early development where one deployment serves one operator and their assets.

Serving a different person or organization should not require row-level isolation
inside one database unless the product explicitly chooses shared multi-tenant
hosting later. Retrofitting scope after external callers depend on global IDs is
expensive (primary keys, unique constraints, idempotency namespaces, changefeed
filtering, and auth semantics).

## Decision

- **Single-tenant by design:** one Atlas Core deployment serves one operator
  context. All data in that deployment belongs to that context.
- **Isolation between operators or orgs = separate deployments** (separate
  compose stack, database, and filesystem volume), not shared-database
  multi-tenancy.
- **No placeholder tenant columns** until there is a concrete product decision to
  support multiple tenants in one Postgres instance.
- **Future multi-tenant work** (if ever needed) must pick an isolation unit name
  (`tenant_id`, `workspace_id`, or similar) and apply it consistently across auth,
  schema keys, idempotency scope, changefeed metadata, and public request shape.

## Consequences

- Global primary keys and unscoped idempotency scopes are acceptable for current
  development and single-operator deployments.
- The public HTTP edge (when it exists) still owns auth and request identity per
  ADR 0001 and ADR 0003; this ADR does not add tenant fields to HTTP or gRPC yet.
- Revisit this record before intentionally running multiple unrelated operators
  on one database or before offering hosted multi-tenant SaaS.

## Supersedes

—

## Superseded by

—
