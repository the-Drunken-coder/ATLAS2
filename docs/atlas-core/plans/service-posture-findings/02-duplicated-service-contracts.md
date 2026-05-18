# Duplicated Service Contracts Between Functions and Datastorage

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real maintainability risk. It is partly intentional because datastorage is an
internal service seam, but the protos currently mirror much of the caller surface
and can drift.

## Evidence

- `atlas-core/proto/atlas/functions/v1/functions.proto:10` through
  `atlas-core/proto/atlas/functions/v1/functions.proto:45` exposes CRUD,
  object-file streaming, task, and observation RPCs on `AtlasFunctionsService`.
- `atlas-core/proto/atlas/datastorage/v1/datastorage.proto:10` through
  `atlas-core/proto/atlas/datastorage/v1/datastorage.proto:51` exposes a nearly
  parallel `DataStorageService`, plus internal-only reconcile and idempotency
  RPCs.
- `docs/atlas-core/design-decisions/0002-service-boundaries-grpc-changefeed.md:42`
  through `docs/atlas-core/design-decisions/0002-service-boundaries-grpc-changefeed.md:54`
  explicitly say datastorage is not a public API and callers must use functions.

## Reasoning

The boundary is conceptually sound: functions owns validation, orchestration,
idempotency semantics, and changefeed publication; datastorage owns persistence.
The risk is that the proto shape looks like two public APIs. Because both services
use the same request/response messages, a future semantic change can appear to be
implemented at both surfaces while only one layer has the right validation,
defaults, error mapping, or side effects.

## Best Fix

Do not publish or document datastorage as a product API. Then either:

- Narrow `DataStorageService` toward storage-port operations that make the
  internal role obvious; or
- Keep the mirrored shape but add contract tests for behavior that must remain
  equivalent and explicit tests proving product-only behavior, such as changefeed
  publication, exists only through functions.

The architectural direction should be captured in an ADR or a short addition to
ADR 0002 before more methods are added.
