# Public API Security and Transport Posture

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real issue for any non-local or shared deployment. The current compose stack
limits host exposure and now keeps datastorage private, but that is a deployment
guard, not an application-layer public security model. The functions server
itself still has no external caller authentication, authorization, TLS, request
identity ingress, or rate-limit contract.

## Evidence

- `atlas-core/services/functions/cmd/atlas-functions/main.go:56` dials
  datastorage with `grpc.NewClient`.
- `atlas-core/services/functions/cmd/atlas-functions/main.go:58` uses
  `insecure.NewCredentials()` for the internal functions-to-datastorage client.
- `atlas-core/services/functions/cmd/atlas-functions/main.go:59` and
  `atlas-core/services/functions/cmd/atlas-functions/main.go:60` attach only the
  internal datastorage bearer interceptors.
- `atlas-core/services/functions/cmd/atlas-functions/main.go:91` constructs the
  product-facing gRPC server with `grpc.NewServer()` and no auth/TLS/rate-limit
  interceptors.
- `atlas-core/docker-compose.yml:66` and `atlas-core/docker-compose.yml:67` bind
  `atlas-functions` to `127.0.0.1`.
- `atlas-core/docker-compose.yml:19` through `atlas-core/docker-compose.yml:53`
  expose `atlas-datastorage` only on Docker networks, not as a host port.
- `atlas-core/services/datastorage/internal/service/auth.go:16` through
  `atlas-core/services/datastorage/internal/service/auth.go:52` enforce only the
  internal datastorage shared token.

## Reasoning

The default stack is not accidentally internet-facing because compose binds the
functions host port to loopback and keeps datastorage internal. The recent
internal bearer token hardens the functions-to-datastorage seam, but it does not
authenticate product callers. Today any process that can reach the functions
socket can call all unary and streaming methods. Internal seam auth must stay
separate from user/workspace auth.

## Best Fix

Define the public edge contract before any non-local deployment or shared
environment:

- Add a functions-layer unary/stream auth interceptor for external callers.
- Decide whether the first supported mode is mTLS, bearer token, or deployment
  gateway auth with verified identity metadata.
- Add request/correlation ID ingress handling and propagate it into logs.
- Keep the datastorage internal bearer token as a service-to-service seam only.
- Add rate limiting at the edge gateway or in a small interceptor once the auth
  identity exists.

This does not require securing the internal datastorage hop with TLS for local
compose immediately. The production story should state whether that traffic is
isolated by deployment networking, encrypted in transit, or both.
