# Atlas Core: Client Sync, List Correctness, and Manifest Simplification

## Status

**Accepted plan** (2026-05). Implementation not yet complete. Authoritative
architecture context: [ADR 0002](../design-decisions/0002-service-boundaries-grpc-changefeed.md)
(amended for SDK sync and list semantics).

This document is the working plan for GitHub issues **#66**, **#69**, **#74**,
**#75**, and related SDK work (Vertical Slice 3). It supersedes issue text that
assumes a durable Postgres mutation log or `mutation_seq` on resources.

## Goals

1. **Product clients use the Atlas SDK** — not raw changefeed or ad hoc list
   pagination semantics.
2. **Strictly complete full sync** — one SDK full pull must not permanently skip
   rows (fixes #74).
3. **Best-effort changefeed in functions only** — in-process hub; no durable log
   in datastorage or Postgres (#66 reframed).
4. **Filesystem-only object manifests** — drop the Postgres manifest JSON cache
   (#69).
5. **Co-located workers (e.g. fusion)** use functions unary list/read APIs; they
   do not depend on the changefeed.

## Non-goals

- Durable or resumable mutation log, outbox table, or multi-day replay
- `mutation_seq` / `changefeed_seq` columns on resource tables
- Explicit list↔stream atomic checkpoint on the server (#75 closed by SDK pattern)
- Idempotency TTL or uniform create-idempotency across all resource types (defer
  until SDK defines retry behavior; see [ADR 0001](../design-decisions/0001-api-boundary-idempotency-versioning.md))
- Slimming `datastorage.proto` before the first composite functions RPC (ADR 0002)
- gRPC health checks, schema audit tables, or migration frameworks

---

## 1. Changefeed (functions, in-memory)

### Decision

`SubscribeMutations` remains a **best-effort live stream** published from
`atlas-functions` after successful mutations. Implementation stays in
`services/functions/internal/changefeed/` (in-process hub). **No** changefeed
persistence in datastorage or Postgres.

### SDK contract

- The SDK opens `SubscribeMutations` for low-latency hints.
- The SDK runs a **strictly complete full list sync** on a timer (default on the
  order of tens of seconds; configurable).
- On disconnect, eviction (`RESOURCE_EXHAUSTED`), or functions restart: SDK
  resubscribes and runs a full list sync.
- End users do not call the changefeed directly; only the SDK does.
- After long offline periods, the SDK does **not** replay days of events—it runs a
  full list sync.

### Server follow-ups (small)

- Keep hub buffer/eviction behavior documented and tested.
- Optional: tune buffer size or error messages for SDK refetch loops (not a
  durable log).

### Issue mapping

| Issue | Outcome |
|-------|---------|
| #66 | Close or rewrite: durable log is out of scope; track SDK + hub docs here |
| #75 | Document-only for raw clients; SDK full sync + stream is the product pattern |

---

## 2. Strictly complete full list sync (#74)

### Problem

Keyset pagination on `ORDER BY updated_at DESC, id ASC` with a page cursor can
**permanently skip** rows inserted during a multi-page list (phantom insert).

### Decision: snapshot watermark on `updated_at`

Full sync (SDK or any caller that requires strict completeness) uses a
**sync watermark**:

1. On the **first** list request of a full sync, capture `sync_watermark` as the
   current UTC time (server clock at request handling).
2. Every page of that sync applies:
   - `updated_at <= sync_watermark`
   - existing keyset cursor predicates within that set
   - `ORDER BY updated_at DESC, id ASC`
3. Rows with `updated_at > sync_watermark` belong to a **later** sync (they did
   not exist at the snapshot boundary).

### Implementation notes

- **Proto/API:** add an optional `sync_watermark` (or `list_as_of`) on list
  requests, or have the server set it on the first page and return it in the
  response for subsequent `page_token` continuations (encoded in the token).
- **Stores:** extend `appendKeysetCursor` / list builders in
  `datastorage/internal/postgres` for entities, objects, tasks, and observations.
- **Within-watermark phantoms:** rows inserted during the sync with
  `updated_at <= sync_watermark` but after page 1 can still be skipped by naive
  keyset. Mitigation (pick one for v1):
  - **Preferred:** run the paginated list for a given watermark inside a single
    Postgres **`REPEATABLE READ`** transaction (snapshot isolation), or
  - **Alternative:** restart the full list from page 1 if row counts or max
    `(updated_at, id)` change before completion.
- **Tests:** concurrent insert/update during multi-page list; assert no permanent
  skips when `sync_watermark` / snapshot mode is used.
- **Incremental lists:** `updated_after` filters for fusion/SDK deltas may
  continue without watermark when not doing a strict full sync; document which
  mode each caller uses.

### Consumers

| Consumer | Mode |
|----------|------|
| Atlas SDK (full cache refresh) | Strict: watermark + snapshot transaction or restart rule |
| Atlas SDK (incremental) | `updated_after` + pagination; may use watermark on periodic full sync only |
| `atlas-fusion` (`ListObservations`) | Strict completeness within each multi-page `Fetch` (same list fix) |

---

## 3. Object manifest: filesystem only (#69)

### Decision

- **Canonical manifest:** `objects/{object_id}/manifest.json` on the filesystem.
- **Remove** maintaining `objects.json["manifest"]` / `manifest_version` in
  Postgres as a write path and query cache.
- **Reads:** `GetObjectManifest` and related paths read from the filesystem only.
- **Drop or simplify** `manifest_current` / `manifest_sync_error` on the mutation
  response once dual-write is gone (or keep briefly during transition).
- **Reconcile:** keep ADR 0002 reconcile rules; repair FS manifest and DB object
  row metadata, not a duplicate manifest JSON column.

### Implementation phases

1. Stop writing manifest fields into `objects.json` on success paths.
2. Change reads to filesystem-only.
3. Remove dead cache-update/repair code and tests tied to DB manifest cache.
4. Update protocol/docs that describe manifest cache reserved fields (when protocol
   catches up to Core).

### Historical doc

[vertical-slice-1/SPEC.md](../vertical-slice-1/SPEC.md) still describes dual-write;
this plan is the target behavior.

---

## 4. Atlas SDK (Vertical Slice 3)

### Decision

The SDK owns client sync:

```
┌─────────────┐     SubscribeMutations      ┌──────────────────┐
│  Atlas SDK  │ ◄────────────────────────── │ atlas-functions  │
│  local cache│     List* (full + filter)   │  + changefeed    │
└─────────────┘ ──────────────────────────► └──────────────────┘
       │
       ▼
  Application (current state)
```

- **Merge:** apply stream events to cache; replace or reconcile on full sync.
- **Full sync:** strictly complete paginated list per resource families the SDK
  caches (watermark per §2).
- **Timer:** periodic full sync (e.g. ~30s) in addition to stream-driven updates.
- **HTTP:** public API (VS3) calls functions on the same host; SDK does not expose
  raw changefeed to app developers.

Details: [atlas-sdk/design-principles.md](../../atlas-sdk/design-principles.md).

---

## 5. Typed domain payloads and protocol codegen (#77)

### Direction (parallel track)

- Broader Go structs at Core boundaries for JSON Core interprets (tasks, catalogs,
  observations, etc.).
- **Atlas Core is canonical** when atlas-protocol and Core disagree; update
  protocol schemas from Core, then enable generated Go from atlas-protocol.

Not blocking §1–§4.

---

## 6. Deferred

| Item | Notes |
|------|--------|
| Idempotency TTL / entity+observation create keys (#70) | When SDK defines retries |
| datastorage proto slimming (#76) | Per ADR 0002 forcing function |
| Schema audit table (#71) | Reset-first deploys wipe DB |
| Ops health (#78) | gRPC health, deeper readiness later |
| golangci-lint in CI (#73) | Hygiene |

---

## Implementation order

1. **List snapshot watermark + strict sync tests** (#74) — unblocks SDK and fusion.
2. **Manifest FS-only** (#69) — reduces datastorage complexity.
3. **ADR 0002 + SDK docs** — align agents and issues (#66, #75).
4. **SDK implementation** (VS3) — subscribe + periodic strict full sync + tests.
5. **Typed payloads / protocol catch-up** (#77) — incremental.

---

## Verification

- `go test` for postgres list packages with concurrent insert during paginated list.
- Integration: functions list RPC with watermark across pages.
- Manifest: object write/read without DB manifest keys; reconcile tests updated.
- SDK (when built): simulated slow consumer + eviction + full sync recovers cache.

---

## References

- [ADR 0002 — Service boundaries, changefeed, SDK sync](../design-decisions/0002-service-boundaries-grpc-changefeed.md)
- [ADR 0001 — HTTP idempotency](../design-decisions/0001-api-boundary-idempotency-versioning.md)
- [vertical-slice-3/SPEC.md](../vertical-slice-3/SPEC.md) (historical API sketch)
- GitHub issues: #66, #69, #74, #75
