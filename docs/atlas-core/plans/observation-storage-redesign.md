# Observation Storage Redesign Plan

## Purpose

Redesign observation storage so Core stores one durable current-state observation
plus compact history for the parts that change over time.

The target model is:

- table columns hold query-critical observation lifecycle fields
- observation JSON holds current belief about the observed thing
- observation JSON also holds the latest changing telemetry
- object-file history stores telemetry samples and current-state corrections
- producers keep using the same `observation_id` across short occlusions or
  re-identification events

This is a planning artifact. Do not treat it as implemented behavior until the
checklist below is complete.

## Decisions

- Replace row-level `observed_at` with row-level `started_at` and `ended_at`.
- `started_at` is required.
- `ended_at` is nullable. `null` means the observation is open/active through
  the present.
- Do not store `"now"` as a timestamp value.
- Keep full absolute timestamps in storage. UIs may display time-only values.
- Do not duplicate the observation time window inside observation JSON.
- Replace JSON `state` with the row lifecycle implied by `started_at` and
  `ended_at`.
- Use one JSON `identity` section for stable/current facts. Do not split
  `classification` and `identity`.
- Store latest changing data in JSON under `latest_telemetry`.
- Store archived changing data in the observation history object.
- Store corrections to current belief, such as car-to-truck re-identification,
  as history events/diffs rather than repeated full observation snapshots.

## Current State To Change

Current `model.Observation` has:

- `observation_id`
- `source_asset_id`
- `target_entity_id`
- `observed_at`
- `json`
- version and timestamps

Current Postgres storage has matching columns and indexes, including
`observed_at`.

Current observation JSON allows:

- `state`
- `latest_sighting`
- `sightings_object_id`
- `extra`
- `custom_*`

Current ingest appends one normalized sighting envelope to `sightings.ndjson`,
then writes a row JSON payload with `state`, `latest_sighting`, and
`sightings_object_id`.

## Target Storage Shape

### Observation Row

The row should store:

- `observation_id`
- `source_asset_id`
- `target_entity_id`
- `started_at`
- `ended_at`
- `json`
- `version`
- `created_at`
- `updated_at`

Constraints:

- `started_at` is required.
- `ended_at` is optional.
- if `ended_at` is present, it must be greater than or equal to `started_at`.
- `source_asset_id` remains the producer/source asset.
- `target_entity_id` remains optional until a track/target relationship is
  explicit.

Indexes:

- keep `observations_source_asset_idx`
- keep `observations_updated_at_idx`
- keep `observations_target_entity_idx`
- replace `observations_observed_at_idx` with an index over
  `started_at DESC, observation_id ASC`
- consider `ended_at` indexing only after a real query path needs it

No migration files should be introduced. Follow the existing reset-first
schema-in-code pattern.

### Observation JSON

Observation JSON should represent the current belief/state for this logical
observation:

```json
{
  "identity": {
    "kind": "vehicle",
    "vehicle_type": "sedan",
    "color": "blue",
    "adsb": {
      "icao_hex": "A1B2C3",
      "callsign": "DAL123"
    }
  },
  "latest_telemetry": {
    "observed_at": "2026-05-20T14:10:05Z",
    "kind": "line_of_bearing",
    "data": {
      "observer_latitude": 40.7128,
      "observer_longitude": -74.006,
      "azimuth_deg": 42.1,
      "elevation_deg": 8.5
    }
  },
  "history_object_id": "obs_hist_...",
  "extra": {}
}
```

Rules:

- `identity` is stable/current fact data, not append-only telemetry.
- `identity` can be corrected over time. The row JSON should hold the current
  corrected belief.
- `latest_telemetry` holds the latest changing measurement.
- `latest_telemetry` should reuse the existing sighting kinds where practical:
  `line_of_bearing`, `point`, and `area`.
- line-of-bearing telemetry remains true-north and local-level referenced.
- `history_object_id` points to the observation-owned history object.
- `state`, `latest_sighting`, and `sightings_object_id` should be retired from
  the canonical contract after compatibility strategy is decided.

### Observation History Object

Keep a dedicated object owned by the observation:

- `owner_type = observation`
- `owner_id = <observation_id>`
- `type = observation_history`
- object ID is deterministic from `observation_id`

Use append-only JSON lines. Prefer a single file unless implementation proves
that separate files are clearer:

- `history.ndjson`

Each line should be one event envelope:

```json
{
  "event_id": "obs_evt_...",
  "event_type": "telemetry",
  "recorded_at": "2026-05-20T14:10:05Z",
  "payload": {
    "observed_at": "2026-05-20T14:10:05Z",
    "kind": "line_of_bearing",
    "data": {
      "observer_latitude": 40.7128,
      "observer_longitude": -74.006,
      "azimuth_deg": 42.1,
      "elevation_deg": 8.5
    }
  }
}
```

Supported initial event types:

- `telemetry`: changing measurement sample
- `identity_patch`: correction or enrichment to `identity`
- `lifecycle`: observation opened, closed, or reopened if that event stream is
  useful for audit

Diff format for `identity_patch` should be simple and explicit. Prefer a small
JSON Merge Patch-style payload unless a stronger patch format is required by a
real consumer.

Example identity correction:

```json
{
  "event_id": "obs_evt_...",
  "event_type": "identity_patch",
  "recorded_at": "2026-05-20T14:10:08Z",
  "payload": {
    "previous": {
      "kind": "vehicle",
      "vehicle_type": "truck"
    },
    "current": {
      "kind": "vehicle",
      "vehicle_type": "sedan"
    }
  }
}
```

## API Behavior

### Create Observation

Create should require:

- `observation_id`
- `source_asset_id`
- `started_at`
- valid observation JSON

Create should allow:

- `ended_at = null`
- `target_entity_id = null`
- no history object yet if the caller is only establishing current state

### Update Observation

Update should allow current-state corrections:

- update `identity`
- update `latest_telemetry`
- set or clear `ended_at`
- set `target_entity_id`

If an update changes `identity`, the functions layer should append an
`identity_patch` event before or as part of committing the new current JSON.

### Ingest Observation Telemetry

Rename or reshape `IngestObservationSighting` around telemetry:

- input: `observation_id`, `source_asset_id`, `started_at` when creating, optional
  `ended_at`, optional `target_entity_id`, telemetry payload
- validate source asset and observation refs
- create or verify the observation history object
- append a `telemetry` event to history
- update row `latest_telemetry`
- keep `identity` unchanged unless the request includes an explicit identity
  patch/update

Short target loss should not force a new observation. The producer should keep
using the same `observation_id` when it believes the source stream is still
tracking the same logical target.

### Close Or Reopen Observation

Closing an observation should set `ended_at`.

Reopening can clear `ended_at` only when the same source stream is continuing
the same logical observation. The system should not infer this on its own; the
producer/tracker owns that identity decision.

## Implementation Plan

### Phase 1: Contract Docs And Examples

- Update `docs/atlas-protocol/contracts/resources.md`:
  - replace `state` with row lifecycle language
  - replace `latest_sighting` with `latest_telemetry`
  - replace `sightings_object_id` with `history_object_id`
  - define `identity`
  - define telemetry event history and identity patch history
- Update `docs/atlas-protocol/examples/observations.json`:
  - visual vehicle example
  - ADS-B example
  - line-of-bearing example
  - open observation with `ended_at = null` represented at row/API level, not in
    JSON
- Update `docs/atlas-core/plans/observation-track-system.md` or mark its older
  `observed_at`/`latest_sighting` wording as superseded by this plan.

Verification:

- examples remain valid JSON with `jq empty`
- docs no longer present `observed_at` as the row lifecycle field
- docs no longer present JSON `state` as canonical observation lifecycle

### Phase 2: Shared Model, Proto, And Store Contract

- Replace `ObservedAt` with `StartedAt` and `EndedAt` in
  `atlas-core/services/shared/model/types.go`.
- Replace `observed_at` with `started_at` and `ended_at` in
  `atlas-core/proto/atlas/shared/v1/common.proto`.
- Update generated protobuf code.
- Update `pbconv` conversion tests.
- Update observation list filters:
  - source asset
  - target entity
  - started-at range
  - active/open observations if needed
  - updated-after pagination remains unchanged

Verification:

- `python3 atlas.py codegen`
- focused Go tests for model/proto conversion

### Phase 3: Schema-In-Code And Postgres Store

- Update `atlas-core/services/datastorage/internal/postgres/schema.go`.
- Add `started_at TIMESTAMPTZ NOT NULL`.
- Add `ended_at TIMESTAMPTZ`.
- Add a check constraint for `ended_at IS NULL OR ended_at >= started_at`.
- Remove canonical use of `observed_at`.
- Replace observed-at filtering with started-at filtering.
- Update Postgres tests for create/get/list/update/upsert.

No migration files.

Verification:

- focused Postgres tests with a test database
- local no-DB runs may use `ATLAS_SKIP_POSTGRES_TESTS=true`, but DB-backed proof
  is required before merging

### Phase 4: Protocol Validation

- Update observation JSON validation:
  - allow `identity`
  - allow `latest_telemetry`
  - allow `history_object_id`
  - allow `extra` and `custom_*`
  - reject canonical `state`, `latest_sighting`, and `sightings_object_id` after
    compatibility rules are chosen
- Reuse existing telemetry-kind validation for `latest_telemetry`.
- Keep line-of-bearing azimuth/elevation semantics from the current contract.
- Add tests for identity object shape and invalid telemetry.

Verification:

- invalid observation JSON is rejected before store writes
- valid visual/ADS-B/line-of-bearing examples pass validation

### Phase 5: Functions-Layer Behavior

- Update `CreateObservation`, `UpdateObservation`, and `UpsertObservation` to
  require and validate row `started_at`.
- Remove `deriveObservationFields` behavior that extracts row lifecycle from JSON.
- Add validation that `ended_at >= started_at` when present.
- Update ingest to append telemetry-only events.
- Add identity correction support:
  - compare previous current JSON identity to requested identity
  - append `identity_patch` history event when identity changes
  - update current JSON after history append succeeds
- Keep deterministic event IDs for retry/dedup where practical.

Verification:

- create requires `started_at`
- active observation stores `ended_at = nil`
- close sets `ended_at`
- identity correction appends a history event
- telemetry ingest appends only telemetry event and updates `latest_telemetry`

### Phase 6: API/SDK Surface

- Update public method-contract docs:
  - observation create/update inputs include `started_at` and `ended_at`
  - list options use started-at windows and open/closed filters as needed
  - current JSON shape exposes `identity` and `latest_telemetry`
- Update any service handlers or clients that still expose `observed_at`.
- Preserve `get(id)` for identity reads and `list(options)` for queries.

Verification:

- route/API docs match proto and service behavior
- no stale `observed_at` query docs remain unless explicitly referring to
  telemetry sample time

## Compatibility Questions To Resolve Before Code Changes

- Should existing `state`/`latest_sighting` JSON be rejected immediately or
  accepted behind a transitional compatibility layer?
- Should `history.ndjson` replace `sightings.ndjson`, or should the current file
  name remain with broader event envelopes?
- Should identity patch history be required for all identity changes, or only for
  updates through a dedicated observation-correction operation?
- Should `latest_telemetry.observed_at` remain required for every telemetry
  sample, separate from row `started_at`?
- Should active/open observations be queried via `ended_at IS NULL`, or does the
  initial SDK/API only need started-at windows?

## Completion Criteria

- Observation table uses `started_at` and `ended_at` instead of `observed_at`.
- Observation JSON canonical sections are `identity`, `latest_telemetry`,
  `history_object_id`, `extra`, and `custom_*`.
- Current-state identity corrections are preserved in history.
- Telemetry history does not repeat full stable observation JSON.
- Open observations use `ended_at = null`.
- Existing validation and store tests cover the new model.
- Docs, examples, proto, model, store, functions, and tests agree on field names.
