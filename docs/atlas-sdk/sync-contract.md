# Atlas SDK sync contract (future)

## Status

**Normative future contract** for the Atlas SDK. This is documentation only — no SDK
package is implemented in this repository yet.

Authoritative Core decisions: [ADR 0002](../atlas-core/design-decisions/0002-service-boundaries-grpc-changefeed.md),
[client sync plan](../atlas-core/plans/plan.md).

Exploratory ideas (server-filtered scopes, diffs, etc.) remain in
[features/sync.md](features/sync.md) and are not part of this contract unless promoted here.

## Role of the SDK

- Product applications use the **Atlas SDK**, not raw gRPC.
- End-user code never calls `SubscribeMutations` or list RPCs directly.
- The SDK maintains an in-memory **current-state cache** per configured scope.
- Core remains authoritative for validation, storage, and mutation semantics.

## Two channels

| Channel | Core API | Purpose |
|---------|----------|---------|
| **Hints** | `SubscribeMutations` on `atlas-functions` | Low-latency “something changed” |
| **Truth** | Paginated `List*` RPCs | Strictly complete snapshots |

The changefeed is **best-effort**, in-process in functions, with **no** durable log in
Postgres or datastorage. There is no multi-day replay.

## Full sync (strictly complete)

Periodic full sync is the repair path and the source of truth for cache contents.

1. For each resource family in scope, call the matching `List*` RPC with
   `strict_snapshot = true` and paginate until `next_page_token` is empty.
2. The server captures a **sync watermark** (`updated_at` upper bound) on the first
   page and encodes it in `page_token` for later pages.
3. **Repeat-until-stable:** run step 1 again from page 1; stop when the set of
   resource IDs from a full pass does not grow (cap at 3 passes).
4. Replace or merge the SDK cache from the merged result.

Default interval: on the order of **30 seconds**, configurable.

## Incremental reads

Between full syncs, the SDK may use `updated_after` filters on `List*` **without**
`strict_snapshot` for smaller deltas. Incremental passes are not a substitute for
periodic strict full sync.

## Stream handling

1. Open `SubscribeMutations` after the first full sync (or in parallel; merge carefully).
2. Apply each `MutationEvent` to the in-memory cache (upsert/delete by resource id).
3. On disconnect, `RESOURCE_EXHAUSTED` (subscriber evicted), or functions restart:
   - Close the stream
   - Run a strict full sync (steps above)
   - Resubscribe

v1 uses the **global** mutation stream; the SDK filters events client-side to active
scopes. Server-filtered subscriptions are a possible later enhancement.

## Recovery after offline

After long offline periods, the SDK does **not** replay missed stream events. It runs
a strict full sync and resubscribes.

## Non-goals

- Durable or resumable mutation log
- `mutation_seq` / changefeed sequence columns
- Offline queue or local durable database
- Caller-provided idempotency keys on the public HTTP edge (see ADR 0001)

## Object manifests

Object file indexes live in filesystem `manifest.json` per object. The SDK reads
manifests through Core object APIs; there is no Postgres manifest cache.

## Co-located workers

Internal workers (for example `atlas-fusion`) call functions unary APIs directly and
do not require this SDK sync loop or the changefeed.

## Verification (when SDK is built)

- Simulated stream eviction followed by full sync restores cache.
- Concurrent creates during strict paginated list do not permanently drop IDs
  (repeat-until-stable).
- Cache matches full sync after stream + timer refresh.
