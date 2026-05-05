# Open Question: Function-Layer Changefeed Hook

## Status

Open. Not yet decided. Not yet implemented.

## Context

`SPEC.md:380` lists "future event emission points" as a function-layer responsibility. Slice 1 explicitly excludes Server-Sent Events (`SPEC.md:7, 359, 424`) and the higher-level streaming infrastructure that consumes events.

Slice 2+ will need a live changefeed so that:

- The REST API can serve SSE streams of resource changes.
- ConnectRPC handlers (for our own worker / relay processes) can serve streaming RPCs over the same change source.

Both consumers must observe the *same* stream of mutations. The function layer is the only place that sees every successful mutation — it is the spec-defined mutation boundary (`SPEC.md:382-386`) — so the publisher hook has to live there. Putting the hook in the API layer would miss mutations originating from internal workers; putting it in the store layer would emit phantom events for partial failures that the function layer rolls back.

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
- For multi-step functions, `Publish` fires only at the outer success point (e.g., `CreateObject` after both the database row and filesystem folder land — `function.go:182-195`).
- Idempotent replays (`function.go:156-163, 482-488`) do **not** re-emit; the original effect is what was published.

This is the seam. The actual fan-out hub, the SSE handler, and the ConnectRPC streamer are Slice 2 concerns.

## Why landing the seam in Slice 1 might be right

- The function layer's API would then be **complete** with respect to its eventual responsibilities. Slice 2 work plugs in a real publisher implementation; it does not have to touch every mutation method to add a `Publish` call.
- A no-op publisher changes no runtime behavior. Tests stay green.
- It freezes the event-shape contract early, when the function layer's surface is still small and easy to reason about.
- It is consistent with `SPEC.md:380`'s explicit forward-pointer to "future event emission points."

## Why deferring to Slice 2 might be right

- Slice 1 explicitly excludes SSE and "high-level behavior" (`SPEC.md:7, 359`). A reviewer could read even a no-op publisher as out-of-scope scaffolding.
- Without a real subscriber, the event schema is a guess. Slice 2 work might immediately want to change it (e.g., add resource sub-type, add post-state snapshot), making the Slice 1 seam churn.
- The retrofit cost is bounded: adding the field, the call sites, and the constructor params is a small, mechanical PR.

## Open design questions (do not resolve here)

These will need answers when (or if) the seam lands:

1. **Event payload — minimal or rich?**
   Minimal: `{resource, op, id, at}`. Subscribers re-fetch via `Get*` if they need state.
   Rich: include a JSON snapshot of the post-state record.
   Tradeoff: minimal keeps the publisher cheap and decouples event schema from model schema; rich is friendlier for SSE clients that want push-only state.

2. **Package location.**
   `internal/changefeed` (separate package, clean import graph for future SSE hub and ConnectRPC streamer to import without reaching into `function`).
   `internal/function/changefeed.go` (cohesive with the only current caller).

3. **`Publish` signature: error-returning or fire-and-forget?**
   Fire-and-forget matches typical changefeed semantics and keeps mutation paths clean.
   Error-returning is necessary if a future implementation wants transactional outbox semantics (publish in the same database transaction as the write, exactly-once delivery). A transactional publisher is structurally different — it wraps pg writes — and could be introduced via a separate constructor without changing the existing call sites.

4. **In-process vs cross-process delivery.**
   An in-memory fan-out hub is sufficient if the only subscriber lives in the same binary as the functions (the SSE endpoint).
   A separate worker process needs Postgres `LISTEN/NOTIFY` (publish in the same tx as the write) or an outbox table polled by the worker.
   The `Publisher` interface is the same in both cases; only the implementation changes. This decision can be deferred until a second process actually exists.

5. **Manifest cache-sync partial failure.**
   `UpdateObjectManifest` (`function.go:319-325`) accepts a defined partial-failure mode where the filesystem manifest writes successfully but the database cache update fails, surfacing `MANIFEST_CACHE_SYNC_ERROR`. Per `SPEC.md:331-332`, the filesystem is the source of truth. Should this case still emit a change event? Argument for yes: the authoritative state changed. Argument for no: callers see an error result, and the database cache is stale until the reconciler runs.

## Recommendation

Not made. This document exists to keep the question visible while Slice 1 finishes. Decide before opening Slice 2 scope so the SSE and ConnectRPC streaming work can plan around either outcome.
