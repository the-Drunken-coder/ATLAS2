# Observations

## Purpose

Manage observation records.

Observations may become high-volume data. The SDK should expose method families
that let callers ask for useful slices of observation data without forcing them
to manually scan everything.

## Method Families

- create
- get
  - by ID
  - by started-at time window
  - by latest-telemetry recency
  - by source asset
  - by related track or target
  - by source asset and related track or target
- list
- update
- delete
- ingest telemetry (`IngestObservationTelemetry`)

## Get And List

`get` retrieves observations by intent.

It should support:

- a known observation ID
- observations whose logical stream started within a time range (`started_at`)
- observations with recent telemetry (`latest_telemetry_at`)
- open observations (`ended_at` unset)
- observations produced by one source asset
- observations related to a specific track or target
- observations from one source asset about one track or target

`list` is the general browsing method and should support bounded results.

High-volume consumers should prefer specific `get` options instead of listing
all observations and filtering locally.

## Time Basis

Row fields own lifecycle and indexed recency:

- `started_at` — when the logical observation / source stream began
- `ended_at` — when it closed (`null` = open)
- `latest_telemetry_at` — newest telemetry sample known to Core
- `latest_identity_at` — newest identity change known to Core

Telemetry sample time lives in `json.latest_telemetry.observed_at` and in
`history.ndjson` telemetry events. Do not use row `started_at` as a substitute
for sample time.

Core models `source_asset_id` and `target_entity_id` as promoted fields.

## Smart Observation Reads

Observation reads should be smart at the SDK/API boundary.

That means:

- callers use intent-shaped methods instead of manually assembling broad scans
- the SDK does not hide expensive unbounded reads behind convenient names
- API/Core implementation can add indexes or promoted fields as needed
- high-volume query methods should be designed before callers depend on
  inefficient local filtering

## Notes

Do not add specialized observation reporting helpers until the public SDK/API
observation and object-content paths are stable.
