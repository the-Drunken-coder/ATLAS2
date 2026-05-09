# Slice 2 JSONB Component and Section Contracts

## Purpose

This document defines the JSON sections, component shapes, and minimum field
constraints used by Vertical Slice 2 JSONB validation.

`SPEC.md` defines the validation system, write-path integration, operation
context, and package boundaries. This document defines the resource-local JSON
contracts that validators enforce.

If this document and `SPEC.md` disagree, update the contract and spec together
before implementation.

## Resource JSON Families

Slice 2 validates four JSON families:

- Entity JSON
- Task JSON
- Observation JSON
- Object JSON

Top-level resource identity, ownership, status, timestamps, and version fields
stay in database columns. Caller-owned JSON sections must not duplicate those
promoted fields at the top level.

Unknown top-level keys are rejected unless listed here or prefixed with
`custom_*`.

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

`components` and `extra` must be objects when present. `extra` is optional and
normalizes to `{}` when omitted. `components` is required whenever the entity
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
- `coordinates`: array

Constraints:

- Slice 2 only checks the basic GeoJSON-style envelope
- full geometry topology validation is deferred

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
- use the canonical numeric limits in `docs/vertical-slice-2/SPEC.md`
  ("Validation limits"), including the stricter `custom_*` bounds
- immediate child keys must not duplicate promoted fields, known top-level
  sections, or known component names; nested vendor metadata may reuse common
  words such as `status`

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
- `extra` is optional and normalizes to `{}` when omitted

Cross-resource command checks live in the function layer, not pure blob
validation.

## Observation JSON Sections

Observation JSON allowed top-level keys:

- `state`
- `latest_sighting`
- `sightings_object_id`
- `extra`
- `custom_*`

| Section | Required on create | Required on full update | Notes |
| --- | --- | --- | --- |
| `state` | yes | yes | `active`, `inactive`, or `ended` |
| `latest_sighting` | no | no | Envelope only in Slice 2 |
| `sightings_object_id` | no | no | Points to history object |
| `extra` | no | no | Extension data |
| `custom_*` | no | no | Bounded extension data |

Future patch-style updates validate touched sections first, then validate the
resulting full observation JSON before persistence.

Minimum `latest_sighting` envelope:

```json
{
  "observed_at": "2026-01-01T00:00:10Z",
  "kind": "line_of_bearing",
  "data": {},
  "extra": {}
}
```

Envelope constraints:

- `observed_at` is required and must be RFC 3339
- `kind` is required and must be a non-empty string
- `data` is required and must be an object
- `extra` is optional and must be an object when present
- kind-specific validation is deferred except `line_of_bearing` sightings may
  omit the range field (range is optional)

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
| `document` | none | `content_type`, `extra` | `manifest`, `manifest_version` |

`log` constraints:

- `log_type` must be a string when present
- `started_at` must be RFC 3339 when present
- `ended_at` must be RFC 3339 when present

`photo` constraints:

- `content_type` must be a string when present
- `captured_at` must be RFC 3339 when present
- `width_px` must be a positive integer when present
- `height_px` must be a positive integer when present

`document` constraints:

- `content_type` must be a string when present
- document payload lives in object files, not `object.json`
- the command catalog is stored as a `document` object with `object_id =
  command_catalog`; there is no separate `command_catalog` object type
- command catalog JSON is a keyed command map:

```json
{
  "commands": {
    "move_to_location": {
      "parameters_schema": {
        "type": "object",
        "properties": {},
        "required": [],
        "additionalProperties": false
      }
    }
  }
}
```

- each command entry must include `parameters_schema`

## Deferred Contracts

- Full sighting kind validation
- Full command catalog materialization
- Full object subtype metadata
- Full geometry topology validation
- Generated SDK types
