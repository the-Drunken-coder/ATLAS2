# AtlasFunctionsService Server God Object

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real maintainability issue, not a behavior bug. This is low urgency unless API
surface growth or merge conflicts make the file painful to work in.

## Evidence

- `atlas-core/services/functions/internal/service/server.go` is 719 lines.
- `atlas-core/services/functions/internal/service/server.go:25` through
  `atlas-core/services/functions/internal/service/server.go:43` register both
  `AtlasFunctionsService` and `ChangefeedService` on one server type.
- `atlas-core/services/functions/internal/service/server.go:168` through
  `atlas-core/services/functions/internal/service/server.go:343` include object
  CRUD, manifest, and file streaming handlers.
- `atlas-core/services/functions/internal/service/server.go:475` through
  `atlas-core/services/functions/internal/service/server.go:497` include
  changefeed subscription behavior.
- `atlas-core/services/functions/internal/service/server.go:500` onward includes
  streaming helper internals in the same file.

## Reasoning

The single server type is acceptable for gRPC registration, but the file mixes
resource handlers, file streaming plumbing, default timestamp helpers, and
changefeed streaming. That makes review harder and increases merge conflicts as
the API grows.

## Best Fix

Split files without changing behavior when this area is next touched heavily:

- `server.go`: type, constructor, registration, shared helpers.
- `entity_server.go`
- `object_server.go`
- `object_streaming.go`
- `task_server.go`
- `observation_server.go`
- `changefeed_server.go`

Keep the same `Server` type so generated gRPC registration and tests do not need
architectural changes.

## Resolved

2026-05-18: Split `atlas-core/services/functions/internal/service/server.go` into
`server.go`, `entity_server.go`, `object_server.go`, `object_streaming.go`,
`task_server.go`, `observation_server.go`, and `changefeed_server.go` with no
behavior change.
