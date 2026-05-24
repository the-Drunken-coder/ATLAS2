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
incremental migration. **Time and effort are not constraints** for this work:
implement the target design as if the old observation stack never existed.

### Implementation posture

- **Rewrite, do not shim.** Replace observation model, schema, validation,
  functions ingest, proto/RPC names, tests, examples, and SDK docs in one
  coherent pass. Do not keep dual code paths, adapters, or “temporary” bridges to
  old field names.
- **Delete old concepts outright.** Remove `ObservedAt`, `deriveObservationFields`,
  `state` / `latest_sighting` / `sightings_object_id`, `sightings.ndjson`,
  sighting-specific helpers (`generateSightingID`, `observationJSONForIngest` as
  written today), and filters/RPCs keyed on row `observed_at`. Do not deprecate
  them in comments—delete and replace with the new lifecycle, telemetry, and
  history event model.
- **Phases are sequencing, not compatibility releases.** The phases below are
  doc → contract → store → validation → functions → SDK order for review and
  verification. Each phase should land **only** the new shapes. There is no
  “phase where both models work.”
- **Schema-in-code is the new table definition.** For `observations`, define the
  target columns and indexes in the primary `CREATE TABLE` block. Do not preserve
  `observed_at` in `schemaSQL` or add upgrade SQL to keep the old column alive.
  Operators reset Postgres and object volumes when this lands
  ([ADR 0005](../design-decisions/0005-reset-first-schema-in-code.md)).
- **Tests and goldens follow the new contract only.** Replace protocol invalid
  goldens that encode the old model (`observation-missing-state`, etc.) with
  failures for the new rules. Rewrite service and functions tests against
  `started_at`, `history.ndjson` events, and ingest reconciliation—not patched
  copies of sighting ingest tests.
- **RPC and SDK naming should match the new model.** Prefer renaming ingest to
  telemetry-oriented names (for example `IngestObservationTelemetry`) rather than
  leaving `IngestObservationSighting` as a long-lived alias.

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
- `base_observation_version` — row `version` the operation expected before commit

The current row `version` lives on the observation row only. History lines are
append-only and immutable: do **not** store `result_observation_version` in
`history.ndjson` because the row is updated **after** append, so the result
version is unknown at append time. Reconciliation uses deterministic `event_id`,
`base_observation_version`, and the current row `version` read on retry. If
implementation later needs pending-operation state, keep it outside immutable
history (for example an operation or idempotency record), not as mutable fields
inside a JSONL line.

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

Initial identity on create (`previous` null):

```json
{
  "event_id": "obs_evt_...",
  "event_type": "identity_patch",
  "recorded_at": "2026-05-20T14:10:00Z",
  "effective_at": "2026-05-20T14:10:00Z",
  "observation_id": "obs_...",
  "base_observation_version": 0,
  "payload": {
    "previous": null,
    "current": {
      "kind": "vehicle",
      "color": "blue"
    }
  }
}
```

Identity correction example:

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

Prefer a small explicit diff in `payload` (e.g. `previous` / `current`, with
`previous` null or `{}` for the first identity) unless a consumer requires a
stronger patch format.

#### Deterministic `event_id`

Derive `event_id` from stable inputs so retries do not duplicate history lines,
for example:

- hash of `observation_id`, `event_type`, `observed_at` or `effective_at`, and a
  canonical serialization of the payload (or a caller-supplied idempotency key
  when the ingest API provides one)

Document the chosen formula in functions-layer code and tests.

#### History Object And Reconstructability

- An observation with no event-backed state does **not** need a history object
  yet.
- If `CreateObservation` includes initial `identity`, Core must create or verify
  the history object and append an initial `identity_patch` event (with
  `previous` null or `{}` and `current` equal to the supplied identity) before or
  as part of committing the row. Set `latest_identity_at` from that event's
  `effective_at`.
- If create supplies no `identity`, no history object is required until the first
  `telemetry`, `identity_patch`, or `lifecycle` event.
- **Reconstructability applies to event-backed state only:** if the row JSON
  contains `identity` or `latest_telemetry`, there must be corresponding history
  events (`identity_patch` and/or `telemetry`). Replay those events in order to
  rebuild current belief and latest telemetry when needed.

## Write Path And Failure Semantics

Object-file append and Postgres row update are **not one transaction**. The
plan assumes explicit ordering and reconciliation.

Recommended flow (telemetry ingest or identity update):

1. Validate request (asset, observation refs, JSON, versions, timestamps).
2. Create deterministic `event_id` (and operation id if separate).
3. Ensure `observation_history` object exists (`history_object_id` set on row JSON
   when first needed).
4. Append event to `history.ndjson` (with `base_observation_version` from the row
   read before append).
5. Update observation row: current JSON, `latest_telemetry_at` and/or
   `latest_identity_at`, increment `version`, set `updated_at`.
6. If row update fails after a successful append, **retry/reconcile** using
   `event_id`, `base_observation_version`, and the current row `version`: re-read
   the row, re-apply state from the event (or skip if already applied), do not
   append a second line with the same `event_id`.

Do not pretend file append and row update are atomic. Tests must cover:

- append succeeds, row update fails → retry reconciles without duplicate history
- row update succeeds after transient failure
- version conflict on row update returns a clear error with enough context to retry

Event-backed row state should be reconstructable from `history.ndjson` when
needed (replay `telemetry` and `identity_patch` events in order).

## API Behavior

### Create Observation

Requires:

- `observation_id`, `source_asset_id`, `started_at`
- valid observation JSON without caller-supplied `latest_telemetry`

Allows:

- optional initial `identity` in JSON
- `ended_at = null`, `target_entity_id = null`, `extra`, `custom_*`
- no `latest_telemetry` on create — **rejected** if supplied

Behavior:

- If `identity` is present: create or verify the history object, append initial
  `identity_patch` (`previous` null or `{}`), set `latest_identity_at`, then
  commit the row with `identity` and `history_object_id`.
- If no `identity`: no history object until the first telemetry, identity patch,
  or lifecycle event.

### Update Observation

Allows:

- update `identity` (append `identity_patch`, bump `latest_identity_at`)
- set or clear `ended_at`, set `target_entity_id`, update `extra` / `custom_*`

Does not allow:

- caller-supplied `latest_telemetry` — telemetry must enter through ingest

Any `identity` change must append an `identity_patch` event before or as part of
committing the new current JSON.

### Ingest Observation Telemetry

Replace `IngestObservationSighting` with a telemetry-named RPC (for example
`IngestObservationTelemetry`). Do not keep the old RPC name as an alias. This is
the **only** path for `latest_telemetry`, `latest_telemetry_at`, and telemetry
history events.

Input:

- `observation_id`, `source_asset_id`, telemetry payload
- `started_at` when creating a missing observation row
- optional `ended_at`, `target_entity_id`

Behavior:

- create observation row if missing (when `started_at` is supplied)
- ensure history object exists
- append `telemetry` event to `history.ndjson`
- update row `json.latest_telemetry` and `latest_telemetry_at`
- leave `identity` unchanged unless the request includes an explicit identity
  update (which follows the identity-patch path)

### Close Or Reopen Observation

- Close: set `ended_at` (optional `lifecycle` event for audit).
- Reopen: clear `ended_at` only when the producer confirms the same logical stream
  continues; Core does not infer this.

## Implementation Plan

### Blast radius (rewrite in place)

Touch and replace—not extend—the following. Nothing here should retain the old
observation contract after completion:

| Area | Current anchors | Rewrite action |
| --- | --- | --- |
| Row model | `model.Observation.ObservedAt` | New lifecycle/recency fields only |
| Postgres | `schema.go` `observations`, `observation_store.go` | New columns/indexes; drop `observed_at` |
| Proto / codegen | `common.proto` `Observation`, `ObservationFilter`, ingest RPC | Field swap + new filters; rename ingest RPC |
| Conversion | `pbconv` observation helpers | Match proto; remove `observed_at` paths |
| Store filters | `ObservationFilterState`, `WithObservationObservedAt*` | `started_at`, `latest_telemetry_at`, open/closed |
| Functions | `observation.go` (ingest, derive, sighting JSON builders) | Event append + row update + reconciliation |
| gRPC surface | `observation_server.go`, datastorage service RPCs | Same contract as proto |
| Protocol | `observation.schema.json`, `custom_rules.go`, invalid goldens | New canonical JSON; new history event schema |
| Examples / docs | `observations.json`, `resources.md`, SDK `observations.md` | Examples and read APIs for new fields |
| Tests | `function_test.go`, postgres observation tests, `pbconv_test`, protocol tests | Rewrite expectations; no dual assertions |

`observation-track-system.md` track/provenance sections stay authoritative for
their scope; observation-input sections are superseded by this plan only.

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

- **Rewrite** `observation.go` around the write-path section above—not by patching
  `deriveObservationFields` or `observationJSONForIngest`. Delete sighting-era
  helpers and implement create/update/ingest/close against `history.ndjson` events.
- Require row `started_at`; reject `latest_telemetry` on create; initial
  `identity` on create appends `identity_patch` with `previous` null.
- Rename ingest RPC and constants to telemetry/history naming; file constant
  `history.ndjson` only.
- Append-then-row-update with `event_id` reconciliation (no
  `result_observation_version` in history); telemetry only via ingest; row
  `latest_telemetry_at` / `latest_identity_at` from event timestamps.
- Revisit whether ingest should call `UpsertObservation` or explicit
  create-or-update with version checks; either way, behavior must match the new
  contract, not upsert-over-old-JSON.

Verification: tests for create with/without identity, create rejects telemetry,
  ingest creates row when missing, close/reopen, identity patch, append/update
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
- **`CreateObservation` rejects caller-supplied `latest_telemetry`**; telemetry
  enters only through ingest.
- **Initial `identity` on create** is event-backed by an `identity_patch` with
  `previous` null or `{}`; `latest_identity_at` set from that event.
- History writes only immutable **`history.ndjson`** lines with `telemetry`,
  `identity_patch`, `lifecycle`; **`result_observation_version` is not stored** in
  history events; reconciliation uses `event_id`, `base_observation_version`, and
  current row `version`.
- Telemetry and identity changes timestamped in history (`observed_at` /
  `effective_at`); row copies `latest_telemetry_at` / `latest_identity_at` for
  indexed queries.
- **Event-backed row state** (`identity`, `latest_telemetry` when present) has
  corresponding history events and can be reconstructed by replay.
- Append-then-row-update failure and `event_id` reconciliation covered by tests.
- Docs, examples, proto, model, store, functions, protocol validation, and SDK
  docs agree on field names and file conventions.
