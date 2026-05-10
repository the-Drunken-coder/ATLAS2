# Open Question: Function-Layer Changefeed Hook

## Status

Open. Not yet decided. Not yet implemented.

## Context

`SPEC.md`'s Function layer section lists "future event emission points" as a
function-layer responsibility. Slice 1 explicitly excludes Server-Sent Events in
the Purpose and "What Vertical Slice 1 intentionally does not solve" sections,
and the Store layer says stores should not own SSE or other high-level
streaming behavior.

Slice 2+ will need a live changefeed so that:

- The REST API can serve SSE streams of resource changes.
- ConnectRPC handlers (for our own worker / relay processes) can serve streaming RPCs over the same change source.

Both consumers must observe the *same* stream of mutations. The function layer is
the only place that sees every successful mutation — it is the spec-defined
mutation boundary (see `SPEC.md`'s "Mutation boundary" section) — so the
publisher hook has to live there. Putting the hook in the API layer would miss
mutations originating from internal workers; putting it in the store layer
would emit phantom events for partial failures that the function layer rolls
back.

The question is whether to land the *seam* (interface + no-op default + call sites) in Slice 1 so Slice 2 can attach a real fan-out hub without retrofitting every mutation method, or to defer the entire concept to Slice 2.

## What "the seam" means

Concretely, this would be:

- A `Publisher` interface with a single `Publish(ctx, Event)` method.
- A `NopPublisher` default so no behavior changes in Slice 1.
- A `Publisher` field on each `*Functions` struct, populated via the `New*Functions` constructors.
- A `Publish` call on the success path of every mutation function:
  - `CreateEntity`, `UpdateEntity`, `DeleteEntity`, `UpsertEntity`
  - `CreateObject`, `UpdateObject`, `DeleteObject`, `UpsertObject`, `UpdateObjectManifest`
  - `CreateTask`, `UpdateTask`, `DeleteTask`, `UpsertTask`
  - `CreateObservation`, `UpdateObservation`, `DeleteObservation`, `UpsertObservation`
- For multi-step functions, `Publish` fires only at the outer success point
  (for example `ObjectFunctions.CreateObject` / `ensureObjectCreatedFresh`,
  after both the database row and filesystem folder land).
- Idempotent replays in `ObjectFunctions.CreateObject` and
  `TaskFunctions.CreateTask` are detected from the `idempotency_keys` table
  described in `SPEC.md`. A `completed` record means "this create already
  happened", so the function returns success without calling `Publish` again.
  A missing key, a newly-claimed `pending` key, or a reclaimed `failed` key
  continues down the fresh-create path and would be eligible to publish on the
  outer success point. In the current Slice 1 implementation the create
  functions return only `error`, so a completed replay does not reload or
  return current row/version metadata; it simply avoids duplicating the side
  effect for the caller-supplied resource ID.

This is the seam. The actual fan-out hub, the SSE handler, and the ConnectRPC streamer are Slice 2 concerns.

## Why landing the seam in Slice 1 might be right

- The function layer's API would then be **complete** with respect to its eventual responsibilities. Slice 2 work plugs in a real publisher implementation; it does not have to touch every mutation method to add a `Publish` call.
- A no-op publisher changes no runtime behavior. Tests stay green.
- It freezes the event-shape contract early, when the function layer's surface is still small and easy to reason about.
- It is consistent with `SPEC.md`'s explicit forward-pointer to "future event
  emission points" in the Function layer section.

## Why deferring to Slice 2 might be right

- Slice 1 explicitly excludes SSE and "high-level behavior" in the Purpose and
  Store layer sections. A reviewer could read even a no-op publisher as
  out-of-scope scaffolding.
- Without a real subscriber, the event schema is a guess. Slice 2 work might immediately want to change it (e.g., add resource sub-type, add post-state snapshot), making the Slice 1 seam churn.
- The retrofit cost is bounded: adding the field, the call sites, and the constructor params is a small, mechanical PR.

## Open design questions (do not resolve here)

These will need answers when (or if) the seam lands:

1. **Event payload — minimal or rich?**
   Minimal: `{resource, op, id, at}`. Subscribers re-fetch via `Get*` if they need state.
   Rich: include a JSON snapshot of the post-state record.
   Tradeoff: minimal keeps the publisher cheap and decouples event schema from model schema; rich is friendlier for SSE clients that want push-only state.

2. **Package location.**
   `internal/changefeed` (separate package, clean import graph for future SSE hub and ConnectRPC streamer to import without reaching into `internal/service`).
   `internal/service/changefeed.go` (cohesive with the only current caller).

3. **`Publish` signature: error-returning or fire-and-forget?**
   Fire-and-forget matches typical changefeed semantics and keeps mutation paths clean.
   Error-returning is necessary if a future implementation wants transactional outbox semantics (publish in the same database transaction as the write, exactly-once delivery). A transactional publisher must participate in the same database transaction as the mutation, so it needs access to the active transaction handle (or an equivalent callback scoped to that tx). That is not something you can fix by swapping the `Publisher` implementation in a `New*Functions` constructor alone: you would extend store or function APIs—for example add transaction parameters or tx-scoped publisher hooks to `ObjectStore.CreateObject` and similar store methods, or teach the function layer to accept and thread transaction handles—so existing call sites change along with the constructor wiring. `Publish` still runs on the success path, but the plumbing that ties `Publish` to the writer's tx is an API concern, not a drop-in constructor swap.

4. **In-process vs cross-process delivery.**
   An in-memory fan-out hub is sufficient if the only subscriber lives in the same binary as the functions (the SSE endpoint).
   A separate worker process needs Postgres `LISTEN/NOTIFY` (publish in the same tx as the write) or an outbox table polled by the worker.
   The `Publisher` interface is the same in both cases; only the implementation changes. This decision can be deferred until a second process actually exists.

## Resolved constraint for a future seam

`ObjectFunctions.UpdateObjectManifest` already defines a partial-failure result:
the filesystem write can succeed while the database cache refresh fails, in
which case the function returns `MANIFEST_CACHE_SYNC_ERROR`. When the seam is
implemented, that result must **not** emit a change event. The caller saw a
failed operation and the database cache remains stale until reconciliation
repairs it, so the publish point stays tied to overall function success, not
just the filesystem write. If the reconciler later repairs the cache from the
authoritative filesystem manifest, that reconciliation path is the earliest
place that may emit the corresponding change.

## Recommendation

Not made. This document exists to keep the question visible while Slice 1 finishes. Decide before opening Slice 2 scope so the SSE and ConnectRPC streaming work can plan around either outcome.
