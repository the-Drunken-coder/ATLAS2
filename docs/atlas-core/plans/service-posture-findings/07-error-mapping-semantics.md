# Error Mapping Semantics

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Resolved 2026-05-18. Duplicate create conflicts stay on `AlreadyExists`.
Idempotency-key/resource mismatches use `IDEMPOTENCY_CONFLICT` →
`FailedPrecondition`. Functions gRPC ingress generates or accepts `x-request-id`
and logs unmapped errors with that ID before returning generic `Internal`.

## Evidence

- `atlas-core/services/shared/rpcerrors/errors.go:71` through
  `atlas-core/services/shared/rpcerrors/errors.go:99` maps core errors to gRPC
  statuses.
- `atlas-core/services/shared/rpcerrors/errors.go:75` and
  `atlas-core/services/shared/rpcerrors/errors.go:76` map `model.ErrConflict` to
  `AlreadyExists`.
- `atlas-core/services/shared/rpcerrors/errors.go:86` through
  `atlas-core/services/shared/rpcerrors/errors.go:93` also map field errors with
  code `CONFLICT` to `AlreadyExists`.
- `atlas-core/services/functions/internal/function/object.go:56` through
  `atlas-core/services/functions/internal/function/object.go:60` return a
  `CONFLICT` field error when an idempotency key was already used for a different
  object.
- `atlas-core/services/functions/internal/function/task.go:80` through
  `atlas-core/services/functions/internal/function/task.go:85` do the same for
  tasks.
- `atlas-core/services/shared/rpcerrors/errors.go:99` maps unknown errors to a
  safe generic internal message.

## Reasoning

Most current `ErrConflict` usage appears to mean duplicate primary key on create,
where `AlreadyExists` is normal. The weaker spot is `FieldError{Code:
"CONFLICT"}` for semantic conflicts such as idempotency-key reuse for another
resource. `AlreadyExists` may lead clients to treat a request as a duplicate of
the requested resource rather than a rejected key/resource mismatch. Separately,
generic internal messages are right for client safety, but server logs need a
request ID to make support/debugging practical. The logger can already carry
`request_id`; the missing piece is ingress extraction/generation and consistent
propagation.

## Best Fix

Keep duplicate create conflicts mapped to `AlreadyExists`. Add narrower typed
errors or distinct field codes for semantic conflicts that should map to
`FailedPrecondition`, `Aborted`, or a stable error detail while preserving client
compatibility. Add request/correlation ID ingress at the functions server and
ensure unknown errors are logged server-side with that ID before returning
generic `Internal`.
