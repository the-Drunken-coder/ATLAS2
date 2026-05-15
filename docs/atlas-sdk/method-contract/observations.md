# Observations Method Contract

## Purpose

Manage observation records through intent-shaped methods.

Observations may become high-volume data, so the SDK should avoid convenient
methods that hide broad unbounded reads.

## Method Families

### `atlas.observations.create`

Intent: create one observation record.

Minimum input:

- source asset reference, when available
- observation JSON

Expected output:

- created observation DTO

Mode:

- direct request mutation

### `atlas.observations.get`

Intent: retrieve one observation by identity.

Minimum input:

- observation ID

Expected output:

- one observation DTO

Mode:

- direct request by default
- may read from local cache when a matching observation subscription is active
  and the caller chooses cache-backed reads

### `atlas.observations.list`

Intent: browse bounded observation records or retrieve observations by query
options.

Minimum input:

- optional filters that Core explicitly supports
- required bounds or pagination once the API contract defines them
- time basis when the query is time-based

Expected output:

- bounded observation DTO list
- optional page or cursor metadata once defined

Mode:

- direct request by default
- broad current-state sync should not be the normal answer for high-volume
  observation queries

Supported options should include:

- exact time or nearest time, once semantics are defined
- time window
- source asset
- recent observations from one source asset
- related track or target, once that relationship is explicit in the data model
- source asset and related track or target, once that relationship is explicit
  in the data model

These are options on `atlas.observations.list(...)`, not separate top-level SDK
methods.

### `atlas.observations.update`

Intent: update one observation record.

Minimum input:

- observation ID
- replacement or patch shape, once the API contract chooses update semantics

Expected output:

- updated observation DTO

Mode:

- direct request mutation

### `atlas.observations.delete`

Intent: delete one observation record.

Minimum input:

- observation ID

Expected output:

- deletion acknowledgement or deleted observation identity

Mode:

- direct request mutation

## API Capabilities Required

- observation create, get, list, update, and delete capabilities
- public observation DTOs instead of direct `internal/model` serialization
- Atlas Protocol validation error mapping for observation writes
- bounded observation reads
- source-asset query capability
- time-window query capability once Core defines the time basis and persistence
  support
- track or target query capability once the data model makes that relationship
  explicit
- server-filtered observation subscription scopes

## Current Core Notes

Observation function-layer behavior exists. Core already promotes
`source_asset_id`, so source-asset reads are a natural first query family.

Time-based reads must choose their time basis explicitly. Observation JSON has
`latest_sighting.observed_at`, while Core rows also have server-owned
timestamps.

Do not add specialized observation reporting helpers until the generic
observation and object-content paths are stable.
