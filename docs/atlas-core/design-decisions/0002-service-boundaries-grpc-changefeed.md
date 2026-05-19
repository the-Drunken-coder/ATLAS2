# 0002 Service boundaries, gRPC entrypoint, and changefeed ownership

## Status

Accepted.

## Context

Atlas Core no longer fits comfortably as one binary that owns PostgreSQL,
filesystem object storage, protocol validation, and internal platform mutation
behavior. The restructuring plan requires replaceable service seams, one
internal entrypoint to function behavior, and a changefeed that reflects
successful mutations without introducing an HTTP API yet.

## Decision

- Split Atlas Core into two Go services under `atlas-core/services/`:
  - `datastorage` owns PostgreSQL access, schema setup in code, filesystem
    object storage, and storage-integrity workflows such as object reconcile.
  - `functions` owns validation, orchestration, idempotent mutation semantics,
    and the internal platform gRPC surface.
- Use protobuf/gRPC for the `functions -> datastorage` seam and for the
  internal platform entrypoint into `functions`.
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

### API entrypoints

Until the HTTP API exists, Atlas Core has **no supported remote public product
API.**

- **`atlas-functions` is the internal platform API** — co-located Atlas
  components (future REST gateway, analytics, other on-host services) call it
  over gRPC. It is not the product front door; auth, TLS, and rate limits belong
  on the HTTP edge, not on functions gRPC.
- **External clients must never call `atlas-datastorage` directly** — the
  datastorage gRPC server is a **functions → datastorage** peer seam only.
  Direct callers bypass protocol validation, idempotency orchestration, and
  changefeed publication guarantees on `AtlasFunctionsService`.
- **The public HTTP API is the product edge** — remote clients use REST (planned
  in Vertical Slice 3), which calls `atlas-functions` on the same machine.

Reachability and compose invariants: [0003](0003-internal-api-exposure-posture.md).

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

### Datastorage as CRUD port; functions as platform surface

Today `DataStorageService` and `AtlasFunctionsService` expose a similar CRUD
shape because the functions client adapter calls through to persistence. That
mirror is a transitional layout, not the long-term product model.

| Layer | Owns | Does not own |
|-------|------|----------------|
| **datastorage** | Postgres and filesystem CRUD, schema setup in code, object reconcile, idempotency **storage** primitives (`ClaimIdempotency`, mark completed/failed) | Protocol validation, changefeed publication, composite product flows |
| **functions** | Validation, orchestration, idempotent mutation **semantics**, changefeed, **future composite RPCs** (e.g. asset check-in returning assigned tasks in one call) | Direct SQL or filesystem writes (always via datastorage) |
| **Future REST, WebSocket, radio, or other bridges** | Auth, TLS, rate limits, transport framing on the exposure edge | Duplicated business rules or parallel mutation pipelines; they call functions |

Direction:

- `DataStorageService` should **converge** toward an obvious storage-port shape
  (CRUD, file ops, reconcile, idempotency primitives)—not grow product semantics.
- `AtlasFunctionsService` is the **only** internal platform entry for co-located
  Atlas components. **Non-CRUD RPCs** (composite flows) are added here over time
  and **must not** be added to datastorage.
- Exposure wrappers run on the same machine as the Atlas stack and are thin
  adapters over functions gRPC. They may add transport-specific concerns but must
  not bypass functions for mutations.

Proto slimming to narrow `datastorage.proto` relative to `functions.proto` is
deferred until the first composite functions RPC provides a natural forcing
function. Until then, document this direction and avoid treating datastorage as a
second platform API (see ADR 0003).
