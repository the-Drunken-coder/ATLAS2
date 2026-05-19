# Atlas Core Docs

This folder contains docs for Atlas Core, the Go service implementation that
stores, validates, indexes, and serves Atlas data locally.

Atlas Core docs are implementation-focused. They describe storage, stores,
function-layer behavior, local stack tooling, and how Atlas Core should consume
or enforce Atlas data contracts.

## API layers

Until the HTTP API exists, Atlas Core has **no supported remote public product API.**

| Layer | Role |
|-------|------|
| HTTP REST (VS3, future) | The public HTTP API is the product edge. |
| `atlas-functions` gRPC | `atlas-functions` is the internal platform API for co-located Atlas components on the same machine. |
| `atlas-datastorage` gRPC | External clients must never call `atlas-datastorage` directly. |

Default compose is Docker-internal for functions; host loopback access is opt-in via integration/debug compose (`python3 atlas.py start-debug`) or native deployment. See ADR 0002 and [0003](design-decisions/0003-internal-api-exposure-posture.md).

Deployments are **single-tenant** (one operator context per stack); isolation between unrelated operators is by **separate deployments**, not row-level multi-tenancy. See [0004](design-decisions/0004-single-tenant-deployment-model.md).

## Contents

- `vertical-slice-1/`: storage, stores, function-layer foundation, local stack,
  and runtime basics.
- `vertical-slice-2/`: Atlas Core integration with Atlas Protocol validation,
  including function-layer placement, runtime-check boundaries, error mapping,
  and verification criteria.
- `vertical-slice-3/`: TypeScript Atlas SDK and public HTTP JSON API
  foundation, including client-package shape, transport boundaries, DTO
  mapping, endpoint scope, service events, error mapping, and verification
  criteria.
- `design-decisions/`: Atlas Core architecture decisions that are broader than
  one code change.
- `plans/`: long-horizon restructuring and architecture plans (e.g. multi-service split).

Protocol-level docs live separately under `docs/atlas-protocol/`.
