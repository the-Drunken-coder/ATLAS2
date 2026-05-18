# Datastorageclient Layering Inversion

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real code organization issue.

## Evidence

- `atlas-core/services/functions/internal/datastorageclient/client.go:10`
  imports `services/functions/internal/function` as `functionpkg`.
- `atlas-core/services/functions/internal/datastorageclient/client.go:61` and
  `atlas-core/services/functions/internal/datastorageclient/client.go:62` assert
  `ObjectGatewayClient` implements function-layer interfaces.
- `atlas-core/services/functions/internal/function/object_gateway.go:19` through
  `atlas-core/services/functions/internal/function/object_gateway.go:50` define
  `ObjectGateway`, `ManifestResult`, and streaming gateway interfaces inside the
  function package.

## Reasoning

The transport adapter is under the functions service, so this is not a cross-Go
module import cycle. Still, the dependency direction is awkward: the adapter that
exists to provide a gateway depends on the domain package that consumes the
gateway. That makes it harder to move or test the adapter independently and
encourages future coupling.

## Best Fix

Move the gateway interfaces and small transport-neutral value types to a lower
package, for example `services/shared/gateway` or
`services/functions/internal/gateway`. Then both `function` and
`datastorageclient` depend downward on that package.

Keep the move mechanical: no behavior changes, no proto changes, and interface
names preserved unless there is a clear reason.
