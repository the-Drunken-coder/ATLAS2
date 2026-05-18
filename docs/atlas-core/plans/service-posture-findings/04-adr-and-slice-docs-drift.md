# ADR and Slice Docs Drift From Running Code

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real docs drift. ADR 0001 is specifically about a future HTTP API, but the
current caller-facing gRPC surface already exposes idempotency keys. Vertical
Slice 2 also has stale implementation paths and stale non-goals.

## Evidence

- `docs/atlas-core/design-decisions/0001-api-boundary-idempotency-versioning.md:12`
  through `docs/atlas-core/design-decisions/0001-api-boundary-idempotency-versioning.md:14`
  say idempotency is not part of the public HTTP API.
- `atlas-core/proto/atlas/shared/v1/common.proto:140` through
  `atlas-core/proto/atlas/shared/v1/common.proto:143` expose optional
  `idempotency_key` on `ObjectRequest`.
- `atlas-core/proto/atlas/shared/v1/common.proto:220` through
  `atlas-core/proto/atlas/shared/v1/common.proto:223` expose optional
  `idempotency_key` on `TaskRequest`.
- `atlas-core/services/functions/internal/service/server.go:173` through
  `atlas-core/services/functions/internal/service/server.go:176` pass object
  idempotency keys from the gRPC request into function logic.
- `atlas-core/services/functions/internal/service/server.go:350` through
  `atlas-core/services/functions/internal/service/server.go:353` do the same for
  task creation.
- `docs/atlas-core/vertical-slice-2/SPEC.md:45` through
  `docs/atlas-core/vertical-slice-2/SPEC.md:52` still reference old
  `atlas-core/internal/...` paths.
- `docs/atlas-core/vertical-slice-2/SPEC.md:65` and
  `docs/atlas-core/vertical-slice-2/SPEC.md:66` say streaming RPC/change-event
  delivery/outbox are out of scope, while the current functions proto and ADR
  include `SubscribeMutations`.

## Reasoning

This is not a direct contradiction if ADR 0001 remains scoped only to a future
HTTP JSON API. It becomes confusing because ADR 0002 now declares the functions
gRPC server as the caller-facing entrypoint. A reader cannot tell whether
idempotency is intentionally public on gRPC, intentionally private to future HTTP,
or accidentally leaked.

## Best Fix

Supersede or amend ADR 0001 with the gRPC reality:

- State whether gRPC `idempotency_key` is a stable public contract.
- If yes, document retry/replay semantics for objects and tasks.
- If no, remove/ignore the field at the functions edge before callers depend on
  it.

Also update Vertical Slice 2 to point at `atlas-core/services/...` and clarify
that protocol change-event document validation is separate from the existing
best-effort `SubscribeMutations` delivery stream.
