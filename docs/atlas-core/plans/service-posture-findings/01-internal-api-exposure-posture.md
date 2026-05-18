# Internal API Exposure Posture

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real issue if compose or deployment accidentally widens reach of the internal
platform API. The risk is **accidental exposure** of `atlas-functions` (host
port publish, wrong Docker networks, wide bind)—not missing public-edge auth or
TLS on functions. Functions is intentionally internal; the public HTTP API is
the product edge when it exists.

## Evidence

- `atlas-core/services/functions/cmd/atlas-functions/main.go:91` constructs the
  gRPC server with `grpc.NewServer()` and no auth/TLS interceptors—appropriate
  for an internal platform API, not a product edge.
- `atlas-core/docker-compose.yml` historically published `atlas-functions` on
  host loopback; default layout should publish **no** host ports and attach
  functions only to `atlas-internal` (`internal: true`).
- `atlas-core/services/datastorage/internal/service/auth.go` enforces the
  datastorage internal bearer token on the functions→datastorage seam only.

## Reasoning

`atlas-functions` is the internal platform API for co-located Atlas components.
Until the HTTP API exists, Atlas Core has no supported remote public product API.
Security complexity (auth, TLS, rate limits) belongs on the future REST edge, not
on functions gRPC.

Regressions to guard against: publishing functions on `0.0.0.0` on the host,
attaching functions to non-internal networks, or documenting functions as a
public product API.

## Best Fix

- Enforce compose invariants in `atlas.py architecture-check`: no host ports on
  functions; `atlas-functions` on `atlas-internal` only; `atlas-internal.internal:
  true`.
- Document Docker-internal vs host-native access (integration override for debug).
- Keep the datastorage internal bearer token as a service-to-service seam only.
- Plan REST edge security when the HTTP API is implemented; do **not** add
  functions-layer external auth interceptors for product callers.

Optional later: TLS on functions↔datastorage when deploying off local compose;
state whether that traffic is isolated by networking, encrypted in transit, or both.
