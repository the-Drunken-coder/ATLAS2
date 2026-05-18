# Public API Security and Transport Posture

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real issue. High before this gRPC edge is reachable by anything beyond trusted
local development. The current compose stack limits host exposure, but the
application server itself does not authenticate end callers or use TLS.

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
functions host port to loopback and keeps datastorage internal. That is a useful
deployment guard, not an application-layer product security model. The functions
surface is the supported caller-facing API, and today any process that can reach
that socket can call all unary and streaming methods. Internal seam auth should
stay separate from user/workspace auth.

## Best Fix

Define the public gRPC edge contract before broader use:

- Add a functions-layer unary/stream auth interceptor for external callers.
- Decide whether the first supported mode is mTLS, bearer token, or deployment
  gateway auth with verified identity metadata.
- Add request/correlation ID ingress handling and propagate it into logs.
- Keep the datastorage internal bearer token as a service-to-service seam only.
- Add rate limiting at the edge gateway or in a small interceptor once the auth
  identity exists.

This does not require securing the internal datastorage hop with TLS for local
compose immediately, but the production deployment story should state how that
traffic is isolated or encrypted.
