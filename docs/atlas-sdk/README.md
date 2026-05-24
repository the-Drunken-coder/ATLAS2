# Atlas SDK Planning

**Status: deferred.** SDK and public HTTP API implementation are not active on
`main`. This folder preserves **durable client-facing design principles**, not
ready-to-build method contracts.

Working name: **Atlas SDK** — the future client-facing package for using Atlas
Core’s public API (today assumed TypeScript/npm when work resumes).

## What To Read

- **[design-principles.md](design-principles.md)** — authoritative durable
  decisions for future SDK and public API design.
- **[../atlas-core/design-decisions/](../atlas-core/design-decisions/)** —
  authoritative for current Core architecture, boundaries, and exposure.
- **[../atlas-core/plans/plan.md](../atlas-core/plans/plan.md)** — client sync,
  strict list pagination, manifest simplification (implementation plan).

## Principles (summary)

- Core functions remain authoritative for validation, business rules, storage,
  runtime checks, and protocol enforcement.
- The public API is a bridge over Core functions, not a second business-logic
  layer.
- Design around caller intent, not mirrored internal storage/RPC layout.
- Expect high-level access to service status, entities, objects, tasks,
  observations, and sync/change behavior; keep object metadata separate from
  file/content access; bound observation reads; treat sync as freshness support.

Details and explicit non-decisions: [design-principles.md](design-principles.md).

## Other Material In This Folder

- `features/` and `infrastructure/` — earlier exploratory planning notes. They
  may name methods, scopes, or behaviors that are **not** decided and may be
  stale as Core evolves. Do not use them as an implementation checklist.

When SDK work resumes, regenerate detailed contracts from the then-current API
and Core state.
