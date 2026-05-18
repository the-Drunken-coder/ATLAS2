# Service posture findings (fix order)

Files are numbered **01 → 10** by recommended fix order: **01 first**, **10 last**.

Structural and contract decisions belong at the top so later functional and hygiene work does not get reworked.

| # | Finding | Former # | Tier |
|---|---------|----------|------|
| 01 | [Multi-tenancy, auth, data isolation](01-multi-tenancy-auth-data-isolation.md) | 11 | Structural — document assumption and isolation unit before external use |
| 02 | [Duplicated service contracts](02-duplicated-service-contracts.md) | 02 | Structural — ADR direction before proto surface grows |
| 03 | [ADR and slice docs drift](03-adr-and-slice-docs-drift.md) | 04 | Structural (idempotency contract) + docs hygiene |
| 04 | [Datastorageclient layering inversion](04-datastorageclient-layering-inversion.md) | 07 | Structural — package boundaries before more functions growth |
| 05 | [Schema-in-code release discipline](05-schema-in-code-release-discipline.md) | 03 | Structural — when data must survive releases |
| 06 | [Object lifecycle / reconcile duplication](06-object-lifecycle-reconcile-duplication.md) | 06 | Medium — bounded refactor when touching objects |
| 07 | [Error mapping semantics](07-error-mapping-semantics.md) | 09 | Functional — client-visible status codes and request IDs |
| 08 | [Functions server god object](08-functions-server-god-object.md) | 08 | Hygiene — file split when this area is active |
| 09 | [Postgres test skips vs CI](09-postgres-test-skips-ci-reality.md) | 10 | Hygiene — contributor docs and optional enforce flag |
| 10 | [Changefeed live-hint contract](10-changefeed-live-hint-contract.md) | 05 | Defer — correct today; revisit for multi-instance or durable events |

## Outside this queue

**Internal API exposure posture** was finding **01**; it is now accepted
[ADR 0003](../../design-decisions/0003-internal-api-exposure-posture.md). Maintain via
`python3 atlas.py architecture-check` and review guardrails — not a numbered backlog item here.
