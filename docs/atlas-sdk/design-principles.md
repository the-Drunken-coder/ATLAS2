# Atlas SDK Design Principles

## Status

**Deferred / planning only.** This is not an implementation contract, route map,
or method catalog. SDK and public HTTP API work are not active deliverables on
`main` today.

## Purpose

The Atlas SDK is the eventual **client-facing developer interface** for Atlas
Core: a package (today assumed to be TypeScript/npm) that makes common client
code easy without moving validation, business rules, storage behavior, runtime
checks, or protocol enforcement out of Core.

The public HTTP/API layer should be a **transport and product edge** over the
Core function layer, not a second place where business logic lives.

## Durable Decisions

These principles are meant to stay valid even as Core layout, RPC shapes, routes,
DTOs, and SDK packaging change.

- **Core is authoritative.** Atlas Core functions own validation, runtime
  checks, storage behavior, protocol enforcement, and mutation semantics. The
  SDK validates only for ergonomics; it does not redefine Core rules.
- **API bridges intent to functions.** The public API maps caller intent to the
  function layer. Handlers should not call stores, Postgres, or object-storage
  implementations directly or encode Core-internal models as the public contract.
- **Design from caller intent.** Client-facing methods should reflect what
  applications need to do, not mirror internal storage layout or every internal
  RPC one-for-one.
- **Expected resource families (high level).** Client access is expected to cover,
  in some form: service health/status; entities; objects; tasks; observations;
  and sync/change/event behavior for freshness. Exact operations and shapes are
  not decided here.
- **Objects: metadata vs content.** Object metadata (info, JSON, manifests) and
  object file/content access are different concerns and should stay distinct at
  the product boundary.
- **Observations: bounded reads.** Observation data may become high-volume. Public
  API and SDK design should favor explicit, bounded, query-shaped reads and avoid
  unbounded broad reads hidden behind convenient names.
- **Sync is freshness support, not truth.** Changefeed-style behavior helps
  clients stay fresh; it is not a durable source of truth or a replacement for
  explicit reads when correctness matters.
- **Prefer narrow views for asset-like clients.** Broad current-state sync may
  exist as an option for rich clients, but narrow/scoped views should be the
  default operating model for asset-like consumers.
- **Hide implementation seams.** Public API and SDK surfaces should not expose
  internal gRPC assumptions, store types, Postgres details, filesystem paths,
  object-storage layout, or raw internal model structs.

Authoritative architecture for what exists today: [Atlas Core design
decisions](../atlas-core/design-decisions/) (ADRs), especially [0001](../atlas-core/design-decisions/0001-api-boundary-idempotency-versioning.md),
[0002](../atlas-core/design-decisions/0002-service-boundaries-grpc-changefeed.md),
and [0003](../atlas-core/design-decisions/0003-internal-api-exposure-posture.md).

## Non-Decisions

Explicitly **not** fixed by this document:

- Exact SDK method or module names
- Exact HTTP routes, verbs, or envelopes
- Exact request/response DTO shapes
- Exact sync/subscription scopes, cache modes, or delivery semantics
- Whether subscriptions are server-filtered, client-filtered, or hybrid at the
  HTTP edge (internal changefeed exists today; product-level sync is TBD)
- Multi-language SDKs beyond the current TypeScript-first assumption
- Browser vs Node packaging details beyond “modern `fetch`, injectable for tests”

When SDK implementation resumes, derive detailed method contracts from the
then-current API and Core state. Do not treat older planning notes in
`features/` or `infrastructure/` as frozen specs.

## Relationship To Other Docs

| Question | Where to look |
| -------- | ------------- |
| What is valid Atlas data? | `docs/atlas-protocol/` |
| How does Core work today? | `docs/atlas-core/`, ADRs, `atlas-core/services/` |
| Historical VS3 API slice sketch | `docs/atlas-core/vertical-slice-3/SPEC.md` (historical; ADRs supersede for architecture) |
| Durable client-facing intent | This file and `docs/atlas-sdk/README.md` |
