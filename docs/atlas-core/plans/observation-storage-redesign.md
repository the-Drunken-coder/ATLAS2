# Observation Storage Redesign Plan

## Purpose

Redesign observation storage so Core stores one durable current-state observation
row plus append-only history for telemetry samples and identity corrections.

This plan covers **observation storage, lifecycle, indexed query fields, and
history only**. It does not redesign track entities, track provenance objects, or
fusion output contracts. Those remain in
[`observation-track-system.md`](observation-track-system.md).

The target model is:

- table columns hold query-critical lifecycle and recency fields
- observation JSON holds current belief (`identity`) and latest telemetry
- `history.ndjson` stores timestamped telemetry and identity-change events
- producers keep using the same `observation_id` across short occlusions or
  re-identification when they believe the source stream is still the same logical
  target

This is a planning artifact. Do not treat it as implemented behavior until the
completion criteria below are satisfied.

## Clean-Break Policy

The repository is still in dev. This redesign is a **clean break**, not an
incremental migration.

Decisions:

- **No compatibility layer** for the old observation model.
- **No migration preservation** of old rows, JSON, or object files.
- **No support** for old JSON fields: `state`, `latest_sighting`,
  `sightings_object_id`.
- **No support** for old history file name: `sightings.ndjson`.
- **All old observation data may be wiped or reset** when the new schema and
  contracts land.
- **Reset-first schema-in-code** remains the source of truth
  ([ADR 0005](../design-decisions/0005-reset-first-schema-in-code.md)). Do not
  introduce database migrations.

Old observation rows using `observed_at`, old JSON shapes, and old history files
are invalid after implementation. Callers must adopt the new contract; Core
rejects the old shapes at validation time.

## Decisions

- Replace row-level `observed_at` with `started_at`, `ended_at`,
  `latest_telemetry_at`, and `latest_identity_at`.
- Replace JSON `state` with row lifecycle (`started_at`, `ended_at`).
- Use one JSON `identity` object for current belief only (no per-field timestamps
  in current JSON by default).
- Store latest changing measurement in JSON `latest_telemetry` (with sample
  `observed_at` inside the telemetry envelope).
- Store archived telemetry and identity corrections in `history.ndjson` events.
- Canonical history file is **`history.ndjson`**, not `sightings.ndjson`.
- Do not store `"now"` as a timestamp value.
- Keep full absolute timestamps in storage. UIs may display time-only values.
- Do not duplicate the observation time window inside observation JSON.

## Current State To Change

Current `model.Observation` has:

- `observation_id`, `source_asset_id`, `target_entity_id`, `observed_at`,
  `json`, `version`, `created_at`, `updated_at`

Current Postgres storage has matching columns and indexes, including
`observed_at`.

Current observation JSON allows:

- `state`, `latest_sighting`, `sightings_object_id`, `extra`, `custom_*`

Current ingest appends one normalized sighting envelope to `sightings.ndjson`,
then writes row JSON with `state`, `latest_sighting`, and `sightings_object_id`.

## Target Storage Shape

### Observation Row

The row stores:

| Field | Role |
| --- | --- |
| `observation_id` | Stable logical observation / source-stream ID |
| `source_asset_id` | Producer asset that owns the stream |
| `target_entity_id` | Optional linked track/target entity |
| `started_at` | When this logical observation/source stream began |
| `ended_at` | When this logical observation closed; `null` = open |
| `latest_telemetry_at` | Newest telemetry sample time known to Core |
| `latest_identity_at` | Newest identity correction/change time known to Core |
| `json` | Current belief + latest telemetry + history pointer |
| `version` | Optimistic concurrency for row updates |
| `created_at` | Row creation time |
| `updated_at` | When Core last wrote this row |

Constraints:

- `started_at` is required.
- `ended_at` is optional; when present, `ended_at >= started_at`.
- `source_asset_id` remains the producer/source asset.
- `target_entity_id` remains optional until a track/target relationship is
  explicit.
- `latest_telemetry_at` and `latest_identity_at` are optional until the first
  corresponding event is recorded; functions layer should set them when ingesting
  telemetry or applying identity patches.

#### Row Time Semantics

These concepts are **different** and must not be conflated:

| Concept | Meaning |
| --- | --- |
| **open** | `ended_at IS NULL` — logical observation not closed |
| **closed** | `ended_at IS NOT NULL` — logical observation ended |
| **fresh** | `latest_telemetry_at` is recent (threshold is consumer-defined) |
| **stale** | open (`ended_at IS NULL`) but `latest_telemetry_at` is old |

Field definitions:

- **`started_at`**: when this logical observation / source stream began.
- **`ended_at`**: when this logical observation closed. `null` means open, not
  necessarily fresh. An open observation can be stale.
- **`latest_telemetry_at`**: timestamp of the newest telemetry sample known to
  Core (from event `observed_at` or equivalent in the telemetry payload).
- **`latest_identity_at`**: timestamp of the newest identity correction or change
  known to Core (from `identity_patch` event `effective_at`).
- **`updated_at`**: when Core last wrote the row (any field change).

High-volume consumers (fusion, dashboards, alerting) should query on
`latest_telemetry_at`, `latest_identity_at`, `started_at`, and `ended_at` without
opening object files. Telemetry sample times remain in history events and in
`json.latest_telemetry.observed_at`; the row copies exist for indexed list/query
paths only.

#### Indexes

Keep:

- `observations_source_asset_idx` on `source_asset_id`
- `observations_target_entity_idx` on `target_entity_id`
- `observations_updated_at_idx` on `(updated_at DESC, observation_id ASC)`

Add:

- `(started_at DESC, observation_id ASC)` — lifecycle window queries
- `(latest_telemetry_at DESC, observation_id ASC)` — recency / freshness queries

Consider only when a real query path needs them:

- `(latest_identity_at DESC, observation_id ASC)`
- partial or composite index involving `ended_at` for open-observation lists

Remove canonical use of `observed_at` column and `observations_observed_at_idx`.

No migration files. Apply through reset-first schema-in-code
([ADR 0005](../design-decisions/0005-reset-first-schema-in-code.md)).

### Observation JSON

Canonical top-level sections:

- `identity`
- `latest_telemetry`
- `history_object_id`
- `extra`
- `custom_*`

Example:

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

- **`identity`**: current belief only. Do not attach timestamps to every identity
  field by default. Identity changes are timestamped in history via
  `identity_patch` events; `latest_identity_at` on the row supports queries.
- **`latest_telemetry`**: latest changing measurement. Reuse existing telemetry
  kinds where practical: `line_of_bearing`, `point`, `area`. Each sample keeps
  `observed_at` inside the envelope. Line-of-bearing remains true-north and
  local-level referenced.
- **`history_object_id`**: points to the observation-owned `observation_history`
  object. Core-managed; callers do not invent filenames or ownership.
- **Rejected keys**: `state`, `latest_sighting`, `sightings_object_id` — validation
  must reject these on create/update/ingest.

#### Identity Validation (Initial, Loose)

- `identity`, when present, must be a JSON object.
- `identity.kind`, when present, must be a non-empty string.
- Known sub-objects (e.g. `adsb`) may get stricter validation later.
- Unknown fields inside `identity` are allowed unless there is a concrete reason
  to reject them.

### Observation History Object

Dedicated object owned by the observation:

- `owner_type = observation`
- `owner_id = <observation_id>`
- `type = observation_history`
- object ID is deterministic from `observation_id`

Single append-only file:

- **`history.ndjson`** (not `sightings.ndjson`)

#### Event Types

| `event_type` | Purpose |
| --- | --- |
| `telemetry` | One measurement sample |
| `identity_patch` | Correction or enrichment to current `identity` |
| `lifecycle` | Observation opened, closed, or reopened (audit) |

#### Event Envelope

Each line is one validated event:

```json
{
  "event_id": "obs_evt_...",
  "event_type": "telemetry",
  "recorded_at": "2026-05-20T14:10:05Z",
  "observed_at": "2026-05-20T14:10:05Z",
  "observation_id": "obs_...",
  "base_observation_version": 3,
  "result_observation_version": 4,
  "payload": {
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

Required on every event:

- `event_id` — deterministic for retry/dedup (see below)
- `event_type` — one of `telemetry`, `identity_patch`, `lifecycle`
- `recorded_at` — when Core recorded/appended the event
- `payload` — type-specific body
- `observation_id` — owning observation
- `base_observation_version` — row version the operation expected before commit

Optional when known at append time:

- `result_observation_version` — row version after successful row update

Source/effective timestamps (top-level on the envelope, not inside `payload`):

- **`observed_at`** — for `telemetry` events: when the sample was observed (same
  semantics as `latest_telemetry.observed_at`). Required for telemetry events.
- **`effective_at`** — for `identity_patch` events: when the identity change is
  considered effective. Required for identity patch events. Do not use
  `observed_at` for identity patches; one name per concern avoids ambiguity.

`lifecycle` events may include `effective_at` when the lifecycle transition has a
producer-provided effective time; otherwise `recorded_at` is sufficient.

Telemetry `payload` carries `kind`, `data`, and optional `extra` (no duplicate
`observed_at` inside `payload` when it is already on the envelope).

Identity patch example:

```json
{
  "event_id": "obs_evt_...",
  "event_type": "identity_patch",
  "recorded_at": "2026-05-20T14:10:08Z",
  "effective_at": "2026-05-20T14:10:07Z",
  "observation_id": "obs_...",
  "base_observation_version": 4,
  "payload": {
    "previous": { "kind": "vehicle", "vehicle_type": "truck" },
    "current": { "kind": "vehicle", "vehicle_type": "sedan" }
  }
}
```

Prefer a small explicit diff in `payload` (e.g. `previous` / `current` or JSON
Merge Patch-style) unless a consumer requires a stronger patch format.

#### Deterministic `event_id`

Derive `event_id` from stable inputs so retries do not duplicate history lines,
for example:

- hash of `observation_id`, `event_type`, `observed_at` or `effective_at`, and a
  canonical serialization of the payload (or a caller-supplied idempotency key
  when the ingest API provides one)

Document the chosen formula in functions-layer code and tests.

## Write Path And Failure Semantics

Object-file append and Postgres row update are **not one transaction**. The
plan assumes explicit ordering and reconciliation.

Recommended flow (telemetry ingest or identity update):

1. Validate request (asset, observation refs, JSON, versions, timestamps).
2. Create deterministic `event_id` (and operation id if separate).
3. Ensure `observation_history` object exists (`history_object_id` set on row JSON
   when first needed).
4. Append event to `history.ndjson` (with `base_observation_version`).
5. Update observation row: current JSON, `latest_telemetry_at` and/or
   `latest_identity_at`, `version`, `updated_at`; set `result_observation_version`
   on the appended line when practical or on retry.
6. If row update fails after a successful append, **retry/reconcile** using
   `event_id`: re-read row version, re-apply row state from the event (or skip if
   event already applied), do not append a second line with the same
   `event_id`.

Do not pretend file append and row update are atomic. Tests must cover:

- append succeeds, row update fails → retry reconciles without duplicate history
- row update succeeds after transient failure
- version conflict on row update returns a clear error with enough context to retry

Row current state should be reconstructable from `history.ndjson` when needed
(replay telemetry and identity_patch events in order).

## API Behavior

### Create Observation

Requires:

- `observation_id`, `source_asset_id`, `started_at`, valid observation JSON

Allows:

- `ended_at = null`, `target_entity_id = null`
- no history object until first append-producing operation

### Update Observation

Allows:

- update `identity` (append `identity_patch`, bump `latest_identity_at`)
- update `latest_telemetry` (usually via ingest; direct update only when explicit)
- set or clear `ended_at`, set `target_entity_id`

Any `identity` change must append an `identity_patch` event before or as part of
committing the new current JSON.

### Ingest Observation Telemetry

Reshape `IngestObservationSighting` (or successor RPC) around telemetry:

- input: `observation_id`, `source_asset_id`, telemetry payload; `started_at` when
  creating; optional `ended_at`, `target_entity_id`
- follow write path above: append `telemetry` event, then update row
  `latest_telemetry`, `latest_telemetry_at`, and `json.latest_telemetry`
- leave `identity` unchanged unless the request includes an explicit identity
  update

### Close Or Reopen Observation

- Close: set `ended_at` (optional `lifecycle` event for audit).
- Reopen: clear `ended_at` only when the producer confirms the same logical stream
  continues; Core does not infer this.

## Implementation Plan

### Phase 1: Contract Docs And Examples

- Update `docs/atlas-protocol/contracts/resources.md` for row lifecycle,
  canonical JSON, and `history.ndjson` event shapes.
- Update `docs/atlas-protocol/examples/observations.json` (vehicle, ADS-B,
  line-of-bearing; open observation via row `ended_at = null`, not JSON `state`).
- Update `docs/atlas-core/plans/observation-track-system.md`: mark observation
  storage sections that reference `observed_at`, `latest_sighting`, and
  `sightings.ndjson` as **superseded by this plan** for observation input; leave
  track output / `track_provenance` sections authoritative for their scope.

Verification: examples pass `jq empty`; docs reject old field names as canonical.

### Phase 2: Shared Model, Proto, And Store Contract

- `model.Observation`: `StartedAt`, `EndedAt`, `LatestTelemetryAt`,
  `LatestIdentityAt`; remove `ObservedAt`.
- `common.proto` and codegen: same fields; list filters for started-at range,
  latest-telemetry-at range, open/closed (`ended_at`), existing source/target/
  `updated_after`.
- `pbconv`, `store.ObservationFilterState`, and filter helpers.

Verification: `python3 atlas.py codegen`; focused conversion tests.

### Phase 3: Schema-In-Code And Postgres Store

- `schema.go`: add `started_at`, `ended_at`, `latest_telemetry_at`,
  `latest_identity_at`; drop `observed_at`; add check constraint on `ended_at`;
  add indexes listed above; remove `observations_observed_at_idx`.
- Postgres store list/create/update/upsert and tests.

Verification: DB-backed Postgres tests (not only skip path).

### Phase 4: Protocol Validation

- `observation.schema.json` and `custom_rules.go`: canonical JSON; reject
  `state`, `latest_sighting`, `sightings_object_id`; loose `identity` rules;
  reuse telemetry-kind validation for `latest_telemetry`.
- History append validation for `history.ndjson` event envelopes.

Verification: old shapes fail; new examples pass.

### Phase 5: Functions-Layer Ingest And Reconciliation

- `observation.go`: require row `started_at`; remove JSON-derived lifecycle;
  rename constant to `history.ndjson`; implement append-then-row-update flow and
  `event_id` reconciliation; update `latest_telemetry_at` / `latest_identity_at`
  from event timestamps; identity changes always append `identity_patch`.

Verification: tests for create/close/ingest/identity patch and append/update
failure paths.

### Phase 6: API And SDK Surface

- Public method-contract and SDK docs: row fields, list filters, JSON shape;
  remove `observed_at` as row lifecycle; document `latest_telemetry_at` queries.

Verification: docs match proto and service behavior.

## Resolved Compatibility Decisions

These are fixed for implementation; they are not open questions.

| Topic | Decision |
| --- | --- |
| Old JSON (`state`, `latest_sighting`, `sightings_object_id`) | **Rejected** at validation |
| Old history file (`sightings.ndjson`) | **Not preserved**; only `history.ndjson` |
| Data migration | **None**; reset or wipe old observation data |
| Old row field `observed_at` | **Removed**; use `started_at` / `ended_at` / `latest_telemetry_at` |
| Compatibility layer | **None** |
| Identity patch history | **Required** for every identity change committed through Core |
| Telemetry sample time | **`observed_at`** on telemetry envelope and events; separate from row `started_at` |
| Open observations | Query via **`ended_at IS NULL`** when needed; freshness via **`latest_telemetry_at`** |

## Completion Criteria

- Row uses `started_at`, `ended_at`, `latest_telemetry_at`, `latest_identity_at`;
  old `observed_at` row field removed.
- Row open/closed/fresh/stale semantics documented and testable via column
  values, not JSON `state`.
- Canonical JSON: `identity`, `latest_telemetry`, `history_object_id`, `extra`,
  `custom_*`; old `state`, `latest_sighting`, `sightings_object_id` rejected.
- History writes only **`history.ndjson`** with `telemetry`, `identity_patch`,
  `lifecycle` events including required metadata.
- Telemetry and identity changes timestamped in history (`observed_at` /
  `effective_at`); row copies `latest_telemetry_at` / `latest_identity_at` for
  indexed queries.
- Append-then-row-update failure and `event_id` reconciliation covered by tests.
- Current row state reconstructable from history when needed.
- Docs, examples, proto, model, store, functions, protocol validation, and SDK
  docs agree on field names and file conventions.
