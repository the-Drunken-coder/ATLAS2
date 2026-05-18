# Service posture findings (fix order)

Open backlog items are numbered **08 → 10** by recommended fix order (**08 first**).

Items **01–03** were closed as ADR decisions (see table above). Structural and contract
decisions belong at the top so later functional and hygiene work does not get reworked.

## Resolved (no backlog file)

| Topic | Record |
|-------|--------|
| Single-tenant deployment (no row-level `tenant_id` in schema/RPC) | [ADR 0004](../../design-decisions/0004-single-tenant-deployment-model.md) |
| Mirrored functions/datastorage protos (transitional storage port) | [ADR 0002](../../design-decisions/0002-service-boundaries-grpc-changefeed.md) — *Datastorage as CRUD port* |
| HTTP vs internal gRPC idempotency scope | [ADR 0001](../../design-decisions/0001-api-boundary-idempotency-versioning.md) |
| Datastorageclient layering inversion (`internal/gateway` ports) | [04](04-datastorageclient-layering-inversion.md) — resolved 2026-05-18 |
| Schema-in-code (reset-first; no version ledger) | [ADR 0005](../../design-decisions/0005-reset-first-schema-in-code.md) |
| Object lifecycle / reconcile duplication (`localObjectGateway` removed) | Resolved 2026-05-18 — datastorage owns reconcile; functions tests use gateway port fake only |
| Error mapping semantics (idempotency conflicts + request IDs) | [07](07-error-mapping-semantics.md) — resolved 2026-05-18 |

| # | Finding | Former # | Tier | Status |
|---|---------|----------|------|--------|
| 08 | [Functions server god object](08-functions-server-god-object.md) | 08 | Hygiene — file split when this area is active |
| 09 | [Postgres test skips vs CI](09-postgres-test-skips-ci-reality.md) | 10 | Hygiene — contributor docs and optional enforce flag |
| 10 | [Changefeed live-hint contract](10-changefeed-live-hint-contract.md) | 05 | Defer — correct today; revisit for multi-instance or durable events |

## Outside this queue

**Internal API exposure posture** was finding **01**; it is now accepted
[ADR 0003](../../design-decisions/0003-internal-api-exposure-posture.md). Maintain via
`python3 atlas.py architecture-check` and review guardrails — not a numbered backlog item here.
