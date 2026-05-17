# 0002 Service boundaries, gRPC entrypoint, and changefeed ownership

## Status

Accepted.

## Context

Atlas Core no longer fits comfortably as one binary that owns PostgreSQL,
filesystem object storage, protocol validation, and caller-facing mutation
behavior. The restructuring plan requires replaceable service seams, one caller
entrypoint to function behavior, and a changefeed that reflects successful
mutations without introducing an HTTP API yet.

## Decision

- Split Atlas Core into two Go services under `atlas-core/services/`:
  - `datastorage` owns PostgreSQL access, schema setup in code, filesystem
    object storage, and storage-integrity workflows such as object reconcile.
  - `functions` owns validation, orchestration, idempotent mutation semantics,
    and the caller-facing gRPC surface.
- Use protobuf/gRPC for the `functions -> datastorage` seam and for the single
  caller-facing entrypoint into `functions`.
- Keep unary mutation/query RPCs and the mutation subscription stream on the
  same `atlas-functions` gRPC server and listen address.
- Publish changefeed events from the functions layer after successful outer
  mutations. The first implementation uses an in-process gRPC stream hub and a
  shared event envelope that carries resource, operation, version, and optional
  snapshot data.
- Keep HTTP/JSON and browser SSE as a later API-layer concern. They are not part
  of this restructuring deliverable.

## Consequences

- Datastorage remains the only normal writer of SQL rows and object files.
- Functions can be replaced independently as long as they continue to implement
  the protobuf contracts and mutation publication behavior.
- Local startup and CI now require gRPC code generation before build/compose.
- Cross-service integration shifts toward compose/gRPC verification instead of
  direct same-process package coupling.

### Datastorage gRPC is not a public product API

The `datastorage` gRPC server is an **internal peer seam** for `functions →
datastorage` only. In the default compose layout it is **not** exposed on the
host; it is reachable on the Docker internal network where the functions service
calls it.

Callers and tools MUST NOT treat datastorage as a second public entrypoint.
**Direct clients bypass** the functions layer and therefore bypass protocol
validation, idempotency orchestration, and the changefeed publication guarantees
that unary and streaming mutations on `AtlasFunctionsService` provide. All
external and product-facing traffic belongs on the functions gRPC surface.

### Reconcile Visibility Rules

Datastorage reconcile repairs storage state but must never create new
client-visible rows. The functions service owns object lifecycle; datastorage
only maintains consistency between filesystem and database for objects that
already exist.

Rules:
1. **Invalid folders** (bad names) → deleted
2. **Orphan folders** (no DB row) → quarantined (`.quarantine-<name>-<ts>`), never create DB rows
3. **Valid folders with DB rows** → manifest repaired if missing/corrupt
4. **DB rows without folders** → folder recreated + manifest rebuilt (DB is the authority; missing folders are partial state that reconcile must repair)

### Changefeed Recovery Contract

The changefeed (`SubscribeMutations`) is a best-effort live stream, NOT a durable
or resumable event log. There is no persistent cursor, outbox table, or replay
mechanism. This is by design.

Clients MUST follow this contract:

1. **On connect**: open a subscription and receive live mutations from this point forward.
   No prior events are replayed.
2. **On disconnect**: resubscribe and refetch full current state via unary RPCs.
3. **On RESOURCE_EXHAUSTED** (subscriber fell behind): resubscribe and refetch.
4. **On functions restart**: all subscriptions are lost; resubscribe and refetch.

Never build client logic that depends on receiving every event — the stream is
an optimization for low-latency updates, not a source of truth. Unary RPCs are
the authoritative data path.
