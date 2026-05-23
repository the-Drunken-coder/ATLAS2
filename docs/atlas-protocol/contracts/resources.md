# Atlas Protocol Resource Contracts

## Purpose

This document collects the initial Atlas Protocol resource document contracts.
It was extracted from earlier Atlas Core Vertical Slice 2 planning and should
evolve as protocol-owned documentation.

The goal is to define reusable Atlas-shaped JSON documents without making Atlas
Core the only source of truth. Atlas Core may consume these contracts, but the
contracts should also be useful to tools, agents, clients, simulators, and
future services.

If this document and the files under `../examples/` disagree, treat the mismatch as
a protocol documentation bug and update them together. Machine-checked fixtures
copy from here into `atlas-protocol/examples/` during `npm run build` in the
`atlas-protocol` package; keep both locations aligned.

## Resource Document Families

The initial protocol surface covers four caller-owned resource JSON families
(see [variants](#resource-and-variant-summary) below):

- Entity JSON
- Task JSON
- Observation JSON
- Object JSON

Related protocol documents:

- Command Catalog JSON
- Validation Error JSON
- Change Event JSON

Top-level resource identity, ownership, status, timestamps, and version fields
stay in database columns. Caller-owned JSON sections must not duplicate those
promoted fields at the top level.

Unknown top-level keys are rejected unless listed here or prefixed with
`custom_*`.

## Resource and variant summary

When using the TypeScript validator, each resource family selects a **variant**
for entity and object JSON:

| Resource family | Variant values | Role |
| --- | --- | --- |
| Entity | `asset`, `track`, `geofeature` | Asset tracks supported commands; track requires paired telemetry lat/lon; geofeature requires geometry |
| Object | `log`, `photo`, `command_catalog`, `document` (deprecated), `observation_history`, `track_provenance` | Per-variant allowed top-level fields (see object contracts) |
| Task | (none) | Single task document shape |
| Observation | (none) | Single observation document shape |
| Command catalog | (none) | Catalog root document |

## Entity Component Matrix

| Component | Asset | Track | Geofeature | Required? | Notes |
| --- | --- | --- | --- | --- | --- |
| `supported_commands` | yes | no | no | asset create/full update/upsert | Required for task targeting |
| `telemetry` | yes | yes | no | track create/full update/upsert | Position and motion (required on tracks; optional on assets) |
| `geometry` | no | no | yes | geofeature create/full update/upsert | Static geometry |
| `status` | yes | yes | yes | no | Display posture |
| `heartbeat` | yes | no | no | no | Asset check-in health |
| `health` | yes | no | no | no | Asset health state |
| `communications` | yes | no | no | no | Reachability and radio info |
| `sensor_refs` | yes | no | no | no | Sensor metadata (asset onboard sensors) |
| `fusion_summary` | no | yes | no | no | Track fusion metadata |
| `custom_*` | yes | yes | yes | no | Bounded extension data |

Entity JSON allowed top-level keys:

- `components`
- `extra`
- `custom_*`

`components` and `extra` must be objects when present. `extra` is optional; the
validator does not rewrite documents, and omitting `extra` is valid. Consumers
may treat a missing `extra` like an empty object if they apply defaults.
`components` is required whenever the entity
type has required components for the operation.

## Entity Component Shapes

### supported_commands

Allowed on assets only.

Required fields:

- `commands`: array of command type strings

Constraints:

- each command type must be a non-empty string
- duplicate command types are rejected
- an empty `commands` array is valid, but the asset cannot accept tasks until a
  requested command appears in the array

### telemetry

Allowed on assets and tracks.

Optional fields:

- `observed_at`: RFC 3339 timestamp. Useful when measurement time differs
  from write time; bandwidth-constrained asset updates may omit it.
- `latitude`: number from -90 to 90
- `longitude`: number from -180 to 180
- `altitude_m`: number
- `speed_m_s`: number greater than or equal to 0
- `heading_deg`: number greater than or equal to 0 and less than 360
- `uncertainty_radius_m`: number greater than or equal to 0; horizontal
  uncertainty radius around `latitude`/`longitude`, in meters. Intended for
  display ("the thing is somewhere within this circle"). Typically supplied
  by the data fusion system on tracks.

Constraints:

- if one of `latitude` or `longitude` is present, both must be present
- altitude, speed, and heading do not imply position by themselves
- on tracks (where telemetry is required), `latitude` and `longitude` must
  both be present
- `uncertainty_radius_m` is meaningful only when `latitude` and `longitude`
  are also present

### geometry

Allowed on geofeatures only.

Required fields:

- `type`: non-empty string
- `coordinates`: array, except `GeometryCollection`
- `geometries`: array, only for `GeometryCollection`

Constraints:

- `type` must be a standard GeoJSON geometry type: `Point`, `MultiPoint`,
  `LineString`, `MultiLineString`, `Polygon`, `MultiPolygon`, or
  `GeometryCollection`
- coordinate positions are `[longitude, latitude]` or
  `[longitude, latitude, altitude_m]`
- longitude is from -180 to 180 and latitude is from -90 to 90
- `LineString` geometries require at least 2 positions and must not contain
  repeated adjacent positions (zero-length segments)
- each line in a `MultiLineString` must follow the same rules as `LineString`
- `Polygon` rings require at least 4 positions, must be closed, and must not
  self-intersect
- non-standard shapes such as `Circle` are not valid Atlas Protocol GeoJSON;
  use `custom_*` sections for extension metadata until a versioned extension is
  documented

### status

Allowed on assets, tracks, and geofeatures.

Optional fields:

- `state`: non-empty string
- `label`: string
- `priority`: integer greater than or equal to 0

### heartbeat

Allowed on assets only.

Optional fields:

- `observed_at`: RFC 3339 timestamp. Useful when a heartbeat is buffered or
  relayed after it was emitted; bandwidth-constrained asset updates may omit it.
- `source`: string
- `sequence`: integer greater than or equal to 0

### health

Allowed on assets only.

Optional fields:

- `state`: non-empty string
- `battery_percent`: number from 0 to 100
- `faults`: array of strings

### communications

Allowed on assets only.

Optional fields:

- `links`: array of objects

Each link object may include:

- `type`: non-empty string
- `status`: non-empty string
- `address`: string
- `rssi_dbm`: number
- `snr_db`: number

### sensor_refs

Allowed on assets only. Describes the asset's onboard sensors as identity
information. Detection-side sensor data for tracks belongs in
`fusion_summary` and the object referenced by `fusion_summary.provenance_object_id`.

Optional fields:

- `sensors`: array of objects

Each sensor object may include:

- `sensor_id`: non-empty string
- `type`: non-empty string
- `label`: string
- `object_id`: string
- `mount`: object describing where the sensor is mounted on the asset and how
  it faces relative to the asset body frame

Each `mount` object may include:

- `location`: non-empty string, such as `front`, `rear`, `left`, `right`,
  `top`, `bottom`, or a platform-specific mount label
- `bearing_deg`: number greater than or equal to 0 and less than 360, where
  0 means facing the asset's forward direction
- `elevation_deg`: number from -90 to 90, where -90 means facing straight down
  and 90 means facing straight up
- `roll_deg`: number greater than or equal to 0 and less than 360

### fusion_summary

Allowed on tracks only.

Optional fields:

- `observed_at`: RFC 3339 timestamp
- `source_count`: integer greater than or equal to 0
- `confidence`: number from 0 to 1
- `provenance_object_id`: string

### custom_*

Allowed on entities, tasks, observations, and objects where explicitly accepted.

Constraints:

- must be a JSON object
- use the protocol validation limits below, including the stricter `custom_*`
  bounds
- does not bypass validation for known core sections

## Protocol Validation Limits

These numeric limits are the current protocol defaults:

- max JSON blob size: 64 KiB
- max nesting depth: 16
- max total object fields: 500
- max key length: 100 characters
- max `custom_*` section size: 16 KiB
- max `custom_*` nesting depth: 8
- max `custom_*` key length: 100 characters
- max `custom_*` total fields: 100

## Task JSON Sections

Task JSON allowed top-level keys:

- `description`
- `created_by`
- `components`
- `extra`
- `custom_*`

| Section | Required on create | Required on full update | Patch behavior | Notes |
| --- | --- | --- | --- | --- |
| `components.command.type` | yes | yes | validate if touched, then validate full result | Command type |
| `components.parameters` | yes | yes | validate if touched, then validate full result | Command parameters |
| `components.progress` | no | no | allowed | Runtime progress |
| `components.result` | no | no | allowed | Completion result |
| `components.error` | no | no | allowed | Failure details |
| `extra` | no | no | allowed | Extension data |
| `custom_*` | no | no | allowed within limits | Bounded extension data |

Task section constraints:

- `components` is required for create, full update, and upsert
- `components.command` must be an object
- `components.command.type` must be a non-empty string
- `components.parameters` must be an object
- `components.progress` must be an object when present
- `components.result` must be an object when present
- `components.error` must be an object when present
- `extra` is optional; omitting it is valid (consumers may default it to `{}`)

Cross-resource command checks live in the function layer, not pure blob
validation.

## Observation JSON Sections

Observation lifecycle and recency live on the **row** (promoted fields), not in
observation JSON. Row fields include `started_at`, `ended_at`, `latest_telemetry_at`,
and `latest_identity_at`. Open vs closed is `ended_at IS NULL` vs set; freshness
queries use `latest_telemetry_at`.

Observation JSON allowed top-level keys:

- `identity`
- `latest_telemetry`
- `history_object_id`
- `extra`
- `custom_*`

Rejected keys: `state`, `latest_sighting`, `sightings_object_id`.

| Section | Required on create | Required on full update | Notes |
| --- | --- | --- | --- |
| `identity` | no | no | Current belief only; changes are event-backed |
| `latest_telemetry` | no | no | **Rejected on create**; set only via telemetry ingest |
| `history_object_id` | no | no | Core-managed pointer to `observation_history` object |
| `extra` | no | no | Extension data (cannot be the only top-level section) |
| `custom_*` | no | no | Bounded extension data |

Observation JSON must include at least one of `identity` or `latest_telemetry`
at the top level. `extra` alone (for example `{"extra":{}}`) is not valid.
Clearing `identity` on update is allowed only when `latest_telemetry` is already
present on the observation. Omitting `identity` in a patch-style update or
upsert preserves the existing identity; set `identity` to JSON `null` to clear
it explicitly when telemetry is present.

Telemetry ingest on an **existing** observation does not set or change
`target_entity_id` (Contract A). Any ingest `target_entity_id` that differs from
the stored row—including late-binding when the row is `NULL` and ingest sends a
non-null target—must be rejected with a clear error; this excludes `NULL`→`NULL`
matches (ingest omits or sends `null` while the stored row is `NULL`), which are
no-ops and must not be rejected. Bind or change `target_entity_id` via create
(first row) or `UpdateObservation` only.

Future patch-style updates validate touched sections first, then validate the
resulting full observation JSON before persistence.

Minimum `latest_telemetry` envelope (after ingest):

```json
{
  "observed_at": "2026-01-01T00:00:10Z",
  "kind": "line_of_bearing",
  "data": {},
  "extra": {}
}
```

Telemetry envelope constraints:

- `observed_at` is required and must be RFC 3339
- `kind` is required and must be a non-empty string
- `data` is required and must be an object
- `extra` is optional and must be an object when present
- `kind` must be one of the currently supported telemetry kinds:
  `line_of_bearing`, `point`, or `area`

`line_of_bearing` `data` fields:

- required: `observer_latitude`, `observer_longitude`, `azimuth_deg`
- optional: `observer_altitude_m`, `elevation_deg`, `range_m`,
  `uncertainty_deg`
- `range_m` is intentionally optional so bearing-only telemetry can omit range
- latitude/longitude ranges match telemetry, `azimuth_deg` is greater than or
  equal to 0 and less than 360, `elevation_deg` is from -90 to 90, and
  `range_m`/`uncertainty_deg` are greater than or equal to 0

`point` `data` fields:

- required: `latitude`, `longitude`
- optional: `altitude_m`, `uncertainty_radius_m`
- latitude/longitude ranges match telemetry and `uncertainty_radius_m` is
  greater than or equal to 0

`area` `data` fields:

- required: `geometry`
- optional: `confidence`
- geometry must be a standard GeoJSON `Polygon` or `MultiPolygon`
- `confidence` is from 0 to 1 when present

## Object JSON Shapes

Object JSON allowed top-level keys depend on `object.Type`. All object JSON
allows:

- `extra`
- `custom_*`

Reserved fields for all object types:

- `manifest`
- `manifest_version`

Only the internal manifest cache update path may write reserved fields.

| Object Type | Required Fields | Optional Fields | Reserved Fields |
| --- | --- | --- | --- |
| `log` | none | `log_type`, `started_at`, `ended_at`, `extra` | `manifest`, `manifest_version` |
| `photo` | none | `content_type`, `captured_at`, `width_px`, `height_px`, `extra` | `manifest`, `manifest_version` |
| `command_catalog` | none | `type`, `name`, `description`, `commands`, `extra`, `custom_*` | `manifest`, `manifest_version` |
| `document` | none | `content_type`, `extra` | `manifest`, `manifest_version` |
| `observation_history` | none | `format_version`, `extra` | `manifest`, `manifest_version` |
| `track_provenance` | none | `format_version`, `extra` | `manifest`, `manifest_version` |

`log` constraints:

- `log_type` must be a string when present
- `started_at` must be RFC 3339 when present
- `ended_at` must be RFC 3339 when present

`photo` constraints:

- `content_type` must be a string when present
- `captured_at` must be RFC 3339 when present
- `width_px` must be a positive integer when present
- `height_px` must be a positive integer when present

`document` constraints (deprecated — use `command_catalog`):

- `content_type` must be a string when present
- document payload lives in object files, not `object.json`

`command_catalog` constraints:

- Atlas Core stores command catalogs as objects with `object_type =
  command_catalog` and JSON matching the command catalog schema (`type`:
  `command_catalog`, `name`, `description`, `commands`)
- optional `extra` and bounded `custom_*` sections follow the same protocol limits
  as other object types

`observation_history` constraints:

- `format_version` must be a string when present
- Core-managed observation history is stored in **`history.ndjson`** (append-only)
- each line is one validated history event envelope (`telemetry`, `identity_patch`,
  or `lifecycle`)
- telemetry events require top-level `observed_at`; identity patch events require
  top-level `effective_at`
- every event includes `event_id`, `event_type`, `recorded_at`, `observation_id`,
  `base_observation_version`, and `payload`

`track_provenance` constraints:

- `format_version` must be a string when present
- current Core-managed fusion provenance is stored in
  `fusion-provenance.ndjson`

## Command Catalog JSON

The command catalog document shape is defined in
[`command-catalog.md`](command-catalog.md).

The initial protocol uses the earlier Atlas catalog shape:

- top-level `type`, `name`, `description`, and `commands`
- `commands` is an array
- each command has a unique string `id`
- each command uses the plural `parameters_schema` field
- consumers may derive a keyed lookup by command `id`, but the canonical
  document shape remains the array

## Deferred Contracts

- Runtime command catalog loading
- Full object subtype metadata
- Generated SDK types
