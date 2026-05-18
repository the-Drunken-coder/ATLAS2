# Object Lifecycle and Reconcile Duplication

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real drift vector, currently mitigated by documentation and production wiring.
Severity is medium for maintainability, not an immediate production correctness
bug.

## Evidence

- `docs/atlas-core/vertical-slice-1/SPEC.md:12` through
  `docs/atlas-core/vertical-slice-1/SPEC.md:19` state datastorage owns
  storage-integrity workflows and `localObjectGateway` is for tests and legacy
  local-mode compatibility.
- `atlas-core/services/functions/internal/function/object_gateway.go:52` through
  `atlas-core/services/functions/internal/function/object_gateway.go:68` repeat
  that `localObjectGateway` includes reconcile/manifest repair and should not be
  used for new production code.
- `atlas-core/services/functions/internal/function/object_gateway.go:285` through
  `atlas-core/services/functions/internal/function/object_gateway.go:333`
  implements local reconcile.
- `atlas-core/services/datastorage/internal/service/object.go:276` through
  `atlas-core/services/datastorage/internal/service/object.go:329` implements
  production datastorage reconcile.

## Reasoning

The local and production implementations now appear aligned on the important
policy of quarantining orphan object folders rather than restoring client-visible
rows. The finding is still valid because there are two separate code paths for
filesystem safety, manifest repair, and reconcile policy. If one gets a security
or consistency fix, the other can drift.

## Best Fix

If local-mode compatibility is not a product requirement, make
`localObjectGateway` test-only and enforce production use of the gRPC gateway. If
local-mode remains useful, extract the shared reconcile/manifest policy into a
small package used by both datastorage and the local gateway, with storage
adapters on either side.

Do not do this as opportunistic cleanup inside an unrelated feature branch; it is
a bounded refactor with tests.
