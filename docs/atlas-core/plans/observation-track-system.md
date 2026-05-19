# Observation And Track State Build Plan

## Purpose

Build the two durable ends of the observation-to-track workflow before building
the fusion system in the middle.

In scope:

- observation writes through the functions layer
- seamless observation ingest for producers
- efficient observation reads for fusion input
- append-only observation sighting history
- validation and verification before object-file writes
- track current-state writes through the functions layer
- track provenance/history storage for fusion output

Out of scope for this plan:

- the data fusion algorithm
- task execution, track following, scheduling, or queue ownership
- database migrations

## Current Repo State

Observations are already first-class Core resources:

- `model.Observation` has `observation_id`, `source_asset_id`, `json`,
  `version`, `created_at`, and `updated_at`.
- Postgres persists observations in the `observations` table.
- Datastorage and functions both expose create, get, list, update, delete, and
  upsert observation RPCs.
- Observation JSON is validated in the functions layer through Atlas Protocol.

The current observation contract already supports:

- `state`: `active`, `inactive`, or `ended`
- `latest_sighting`
- `latest_sighting.kind`: `line_of_bearing`, `point`, or `area`
- `sightings_object_id`
- custom extension sections

Tracks are already modeled as entities:

- `Entity.type = track`
- track JSON requires `components.telemetry` with latitude and longitude
- track JSON may include `components.fusion_summary`
- `fusion_summary.provenance_object_id` points to supporting provenance data

Objects and object files already provide the storage primitive needed for
history:

- objects can be owned by observations or entities
- object files can be written, appended, read, listed, and deleted through the
  functions layer
- append supports current-size preconditions, which is enough for caller-side
  append conflict handling

Important distinction: the current drift is about Core object resource types,
not about whether object storage can physically hold arbitrary file bytes. The
object storage layer can store files, but Core should expose dedicated
object/file contracts for observation history so callers do not have to manage
generic file writes by hand.

## Confirmed Gaps

Observation reads are not yet strong enough for data fusion input. The store can
filter only by source asset, `updated_after`, and pagination. The SDK docs call
out reads by time window and related track/target, but the model, proto, and
store do not yet expose those fields.

Observation history is only a pointer today. `sightings_object_id` is documented,
but there is no Core-level helper or documented file convention for the
append-only sighting stream behind it.

The track output end is mostly present, but it needs a narrower contract:
fusion output should upsert a normal `track` entity for current state and append
supporting provenance to an object referenced by
`components.fusion_summary.provenance_object_id`.

There is protocol/Core object-type drift. Protocol docs mention `document`
objects, but Core keeps `command_catalog` as its own object type. Observation
history should not stay a generic `log` if it has a strict Atlas shape. The
intended direction is a dedicated Core-backed observation history object type
and validated file writes through the functions layer.

## Proposed Resource Shape

### Observation Row

Keep the existing observation row as the current state for one observation
source stream or logical observation.

Add promoted fields for query performance:

- `observed_at`: timestamp extracted from `json.latest_sighting.observed_at`
- `target_entity_id`: nullable entity ID for the track/target relationship once
  known

These are row fields, not caller-owned JSON fields. The function layer should
derive `observed_at` from validated JSON and should reject conflicting promoted
fields inside observation JSON.

### Observation History Object

Use a dedicated Core object type owned by the observation:

- `owner_type = observation`
- `owner_id = <observation_id>`
- `type = observation_history`
- `object_id` equals `json.sightings_object_id`

Use one append-only JSON text file:

- `sightings.ndjson`

Each line should be one validated sighting envelope using the same shape as
`latest_sighting`, plus optional ingestion metadata under `extra`. This avoids a
second sighting schema and keeps current-state and history entries aligned.

The filename and format are part of the contract. Callers should not choose
where sighting history goes, write arbitrary files into the object, or mutate the
history structure directly.

### Seamless Observation Ingest

Add a functions-layer operation for pushing one observation sighting into Core.
The producer supplies the sighting payload and source identity; Core performs the
multi-step write.

The operation should:

1. validate the incoming sighting envelope and source asset
2. determine the observation row to update
3. create or verify the `observation_history` object
4. append the sighting to `sightings.ndjson`
5. update the observation row's `latest_sighting` and `sightings_object_id`
6. derive promoted query fields such as `observed_at`
7. publish the resulting observation mutation

If any validation fails, Core should reject the write before changing current
state or appending history. If append succeeds but current-state update fails,
Core must return a clear error and leave enough information for retry or
reconciliation. The first implementation should keep ordering simple and tested
instead of pretending object-file append and row update are one database
transaction.

### Track Current State

Use the existing entity path:

- `type = track`
- `json.components.telemetry` stores current fused position and uncertainty
- `json.components.status` stores display/current posture
- `json.components.fusion_summary` stores fusion metadata

The fusion system should upsert the track entity through `EntityFunctions`.

### Track Provenance Object

Use a dedicated Core object type owned by the track entity:

- `owner_type = entity`
- `owner_id = <track_entity_id>`
- `type = track_provenance`
- `object_id` equals `json.components.fusion_summary.provenance_object_id`

Use one append-only JSON text file:

- `fusion-provenance.ndjson`

Each line should record the fusion input references used for a track update,
including observation IDs, observation versions, observed-at window, and any
fusion confidence/debug metadata the fusion system wants to preserve.

## Build Order

### Slice 1: Queryable Observation Input

1. Add `observed_at` and `target_entity_id` to `model.Observation` and shared
   proto messages.
2. Extend schema-in-code for new nullable observation columns and indexes.
3. Extend observation list filters:
   - `source_asset_id`
   - `observed_at_from`
   - `observed_at_to`
   - `target_entity_id`
   - `updated_after`
4. In `ObservationFunctions`, derive `observed_at` from validated
   `latest_sighting.observed_at` when present.
5. Add focused Postgres, protobuf conversion, and functions tests.

Success criteria:

- a fusion consumer can page observations by observed-at window
- a fusion consumer can narrow by source asset
- a fusion consumer can narrow by assigned target/track once present
- current CRUD behavior remains compatible

### Slice 2: Dedicated Observation History Object

1. Add `observation_history` to Core object types and protocol object variants.
   Status: done.
2. Document `sightings.ndjson` as the only Core-managed history file.
3. Add sighting-envelope validation for appends.
4. Add a functions-layer helper for creating or verifying the history object.
5. Add a functions-layer helper for appending a sighting entry.
6. Keep the object store generic; do not put sighting semantics in the store.

Success criteria:

- an observation can have durable append-only history
- latest sighting and history entries share one envelope shape
- callers do not need to invent object IDs, filenames, or ownership rules
- malformed JSON or malformed sighting entries are rejected before append

### Slice 3: Seamless Observation Ingest

1. Add the functions-layer ingest operation that accepts one sighting.
2. Wire ingest through object creation, validated append, observation upsert,
   promoted-field derivation, and mutation publishing.
3. Add tests for first sighting, subsequent sighting, invalid sighting, and
   append/update failure behavior.

Success criteria:

- a producer can push observation data with one API call
- Core updates the observation link/current entry
- Core archives every valid sighting in the history object
- producers never handle object creation, file naming, append offsets, or
  current-state JSON rewrites

### Slice 4: Track Output Contract

1. Add `track_provenance` to Core object types and protocol object variants.
2. Document track state as current fused state in a normal `track` entity.
3. Document `fusion-provenance.ndjson`.
4. Add focused tests showing a track entity with `fusion_summary` validates and
   stores through the functions layer.
5. Add helper code only if repeated call sites need it; otherwise keep track
   writes on existing entity methods.

Success criteria:

- fusion output can write current track state through functions
- track provenance has a durable append-only location
- consumers can read current track state by listing/getting entities

### Slice 5: Fusion System Integration Point

After the two ends are built, add the fusion worker/service boundary:

- input: observation list filters by observed-at window/source/target
- output: track entity upsert plus provenance append
- recovery: rebuild from observation rows plus history/provenance objects if
  needed

This slice should not change the resource contracts unless implementation
proves a missing field.

## Design Constraints

- Stores persist; the functions layer enforces meaning.
- Generic object-file writes remain available, but observation history and track
  provenance writes should go through typed functions-layer helpers.
- No database migrations. Schema changes go through reset-first schema-in-code
  and compatible `ADD COLUMN IF NOT EXISTS` upgrades where current code already
  uses that pattern.
- Keep observation JSON lean. Do not add top-level JSON relationship fields when
  a promoted query column is the actual contract.
- Do not introduce a separate track-following feature.
- Do not require range for line-of-bearing observations.
