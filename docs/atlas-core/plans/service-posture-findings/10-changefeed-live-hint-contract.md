# Changefeed Live-Hint Contract

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real risk if clients mistake it for an event log, but the current docs, proto
comments, and implementation already state and enforce the intended live-hint
contract. This is not a current code defect.

## Evidence

- `docs/atlas-core/design-decisions/0002-service-boundaries-grpc-changefeed.md:69`
  through `docs/atlas-core/design-decisions/0002-service-boundaries-grpc-changefeed.md:85`
  define `SubscribeMutations` as best-effort, live-only, non-durable, and not a
  source of truth.
- `atlas-core/proto/atlas/functions/v1/functions.proto:47` through
  `atlas-core/proto/atlas/functions/v1/functions.proto:64` repeat that recovery
  contract in the proto.
- `atlas-core/services/functions/internal/changefeed/hub.go:17` fixes the
  subscriber buffer at 32 events.
- `atlas-core/services/functions/internal/changefeed/hub.go:75` through
  `atlas-core/services/functions/internal/changefeed/hub.go:90` publish with
  non-blocking sends and evict slow subscribers.
- `atlas-core/services/functions/internal/service/server.go:475` through
  `atlas-core/services/functions/internal/service/server.go:497` maps subscriber
  eviction to `RESOURCE_EXHAUSTED`.

## Reasoning

The implementation matches the intended contract. It is suitable for UI/cache
acceleration and live hints. It is not suitable for durable processing,
cross-instance fanout, audit trails, or exactly-once client state transitions.
The main risk is not hidden behavior; it is future scope drift where a caller or
engineer treats the stream as stronger than documented.

## Best Fix

Keep the current implementation as long as docs call it a live hint. Before
multi-instance functions or event-log semantics:

- Add an outbox or durable log with cursors.
- Define ordering guarantees, probably scoped per resource rather than global.
- Add a scaling ADR saying the in-process hub is replaced when more than one
  functions instance can serve the same caller population.

No immediate code fix is needed while the current scope remains live hints only.
