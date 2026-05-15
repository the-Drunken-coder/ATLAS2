# Service Method Contract

## Purpose

Expose Atlas Core service status through simple request-mode SDK methods.

Service methods should help clients decide whether Core is reachable and ready
without depending on sync state.

## Method Families

### `atlas.service.live`

Intent: confirm the Atlas Core process is running and responding.

Minimum input:

- none

Expected output:

- liveness status
- server timestamp, if useful

Mode:

- direct request
- no local cache requirement

### `atlas.service.ready`

Intent: confirm Atlas Core is initialized enough to serve normal resource
operations.

Minimum input:

- none

Expected output:

- readiness status
- optional component status for storage, schema, object storage, protocol
  validation, and startup reconciliation

Mode:

- direct request
- no local cache requirement

### `atlas.service.info`

Intent: expose non-secret service metadata useful for SDK diagnostics and
compatibility checks.

Minimum input:

- none

Expected output:

- service name
- API version or contract version, once defined
- optional protocol version, once defined
- optional auth mode summary, without exposing secrets

Mode:

- direct request
- no local cache requirement

## API Capabilities Required

- liveness capability
- readiness capability
- optional service metadata capability
- stable error envelope for transport failures

## Current Core Notes

Atlas Core already has service startup and readiness concepts. Vertical Slice 3
should expose those concepts through the public API without replacing the
existing Docker ready-file behavior.
