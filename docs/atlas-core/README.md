# Atlas Core Docs

This folder contains docs for Atlas Core, the Go service implementation that
stores, validates, indexes, and serves Atlas data locally.

Atlas Core docs are implementation-focused. They describe storage, stores,
function-layer behavior, local stack tooling, and how Atlas Core should consume
or enforce Atlas data contracts.

## Architecture

Authoritative decisions live in `design-decisions/`:

- [0001](design-decisions/0001-api-boundary-idempotency-versioning.md) — HTTP idempotency and row version at the product edge
- [0002](design-decisions/0002-service-boundaries-grpc-changefeed.md) — Service boundaries, gRPC entrypoints, changefeed
- [0003](design-decisions/0003-internal-api-exposure-posture.md) — Compose reachability and exposure
- [0004](design-decisions/0004-single-tenant-deployment-model.md) — Single-tenant deployment model
- [0005](design-decisions/0005-reset-first-schema-in-code.md) — Reset-first schema-in-code

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
- `plans/observation-track-system.md`: durable observation input and track
  output contracts used by external fusion experiments and future integration.
- `design-decisions/`: Atlas Core architecture decisions that are broader than
  one code change.
- `plans/`: long-horizon restructuring and architecture plans (e.g. multi-service split).

Protocol-level docs live separately under `docs/atlas-protocol/`.
