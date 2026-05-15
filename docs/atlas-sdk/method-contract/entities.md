# Entities Method Contract

## Purpose

Manage Atlas entities through the SDK.

Assets, tracks, and geofeatures are entity records. Asset-specific helpers may
exist later as convenience wrappers, but assets are not a separate Core resource
family.

## Method Families

### `atlas.entities.create`

Intent: create one entity record.

Minimum input:

- entity type
- owner or parent fields required by Core
- entity JSON

Expected output:

- created entity DTO

Mode:

- direct request mutation

### `atlas.entities.get`

Intent: retrieve one entity by identity.

Minimum input:

- entity ID

Expected output:

- entity DTO

Mode:

- direct request by default
- may read from local cache when a matching subscription is active and the
  caller chooses cache-backed reads

### `atlas.entities.list`

Intent: browse or load a bounded set of entity records.

Minimum input:

- optional filters that Core explicitly supports
- optional bounds or pagination once the API contract defines them

Expected output:

- entity DTO list
- optional page or cursor metadata once defined

Mode:

- direct request by default
- broad current-state sync may serve rich clients through local cache once a
  matching subscription is active

### `atlas.entities.update`

Intent: update one entity record.

Minimum input:

- entity ID
- replacement or patch shape, once the API contract chooses update semantics

Expected output:

- updated entity DTO

Mode:

- direct request mutation

### `atlas.entities.delete`

Intent: delete one entity record.

Minimum input:

- entity ID

Expected output:

- deletion acknowledgement or deleted entity identity

Mode:

- direct request mutation

## API Capabilities Required

- entity create, get, list, update, and delete capabilities
- public entity DTOs instead of direct `internal/model` serialization
- Atlas Protocol validation error mapping for entity writes
- runtime and not-found error mapping
- subscription/event capability for entity update scopes

## Current Core Notes

Entity function-layer behavior exists. The API should call that layer and rely
on it for Atlas Protocol validation before persistence.
