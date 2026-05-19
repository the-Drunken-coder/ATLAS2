# 0003 Internal API exposure posture

## Status

Accepted.

## Context

Atlas Core splits product behavior across `atlas-functions` (internal platform gRPC)
and `atlas-datastorage` (private persistence gRPC). ADR 0002 defines those
boundaries; this record defines **reachability and exposure** so we do not
accidentally widen the internal platform API or misread intentional gaps as bugs.

The main risk is **accidental exposure** of `atlas-functions`—host port publish,
wrong Docker networks, wide bind, or docs that treat functions as the public
product API—not missing auth or TLS on the functions server itself.

## Decision

- **`atlas-functions` is the internal platform API** for co-located Atlas
  components on the same machine. Until the HTTP API exists, Atlas Core has **no
  supported remote public product API.** See ADR 0002 for entrypoint roles.
- **Default local compose is Docker-internal only:**
  - Publish **no** host ports for `atlas-functions`, `atlas-datastorage`, or
    `postgres` in `atlas-core/docker-compose.yml`.
  - Attach `atlas-functions` only to `atlas-internal` with `internal: true`.
  - Peer containers dial `atlas-functions:8080` on the internal network.
- **Host loopback access is opt-in** for integration, debug, and native
  deployment: `python3 atlas.py start-debug` (integration compose override) or
  services run on the host with loopback bind. Integration overrides must bind
  to `127.0.0.1`, not `0.0.0.0`, on the host.
- **Functions gRPC stays thin on security:** `atlas-functions` may use plain
  `grpc.NewServer()` without product-edge auth, TLS, or rate-limit interceptors.
  Do **not** add functions-layer external auth interceptors to serve remote
  product callers; that belongs on the future HTTP edge (ADR 0001, ADR 0002).
- **Datastorage internal credential** (`ATLAS_DATASTORAGE_INTERNAL_TOKEN`) is a
  **functions → datastorage** service-to-service seam only, enforced on the
  datastorage gRPC server. It is not a substitute for a public product API or
  functions-layer caller auth.
- **CI enforces compose invariants** via `python3 atlas.py architecture-check`:
  no host ports on functions, datastorage, or postgres in default compose;
  `atlas-functions` networks must be exactly `atlas-internal`;
  `networks.atlas-internal.internal` must be `true`.

## Consequences

### Regressions to guard against

- Publishing `atlas-functions` on the host (especially `0.0.0.0`).
- Attaching `atlas-functions` to non-internal Docker networks.
- Documenting or tooling `atlas-functions` gRPC as the public product API.
- Treating missing functions-layer auth/TLS as a product-edge security gap.

`architecture-check` in `atlas.py` enforces compose invariants and ADR link
references in entrypoint docs; reviewers should still treat the items above as
explicit anti-patterns.

### Optional follow-up (not required for local compose)

When deploying off default compose, decide per environment whether
functions↔datastorage traffic is isolated by networking, encrypted in transit
(TLS/mTLS), or both—and document that choice for operators.
