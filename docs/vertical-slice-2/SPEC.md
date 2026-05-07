# Atlas Core Vertical Slice 2: JSONB Validation and Normalization

## Purpose

Vertical Slice 2 makes Atlas Core's caller-owned JSONB blobs meaningful before
they reach persistence.

> No JSON blob reaches a store write unless the function layer validates and
> normalizes it.

Stores remain persistence-only. Stores do not interpret JSON meaning. The
function layer enforces resource semantics before calling stores.

## Scope

Vertical Slice 2 covers function-layer validation and normalization for the
`json` blobs on:

- entities
- objects
- tasks
- observations

It does not add database migrations, generated SDK types, public HTTP API
generation, deep database JSONB `CHECK` constraints, full command execution, or
data fusion.

## Required write-path coverage

Every function-layer write path that can persist a JSON blob must call
`internal/blobvalidation` before calling a store:

- `CreateEntity`
- `UpdateEntity`
- `UpsertEntity`
- `CreateObject`
- `UpdateObject`
- `UpsertObject`
- `CreateTask`
- `UpdateTask`
- `UpsertTask`
- `CreateObservation`
- `UpdateObservation`
- `UpsertObservation`

If a write path is kept public, it must validate. If a write path should not be
supported, remove or hide it instead of leaving an unvalidated bypass.

## Package layout

Create:

```text
atlas-core/internal/blobvalidation/
  validator.go
  errors.go
  common.go
  entity.go
  components.go
  task.go
  observation.go
  object.go
  custom.go
  command_schema.go
```

### File responsibilities

- `validator.go`: public entry points and resource dispatch.
- `errors.go`: field-path validation errors.
- `common.go`: JSON parsing, canonical marshal, promoted-field checks, and
  shared limits.
- `entity.go`: entity resource validation by entity type.
- `components.go`: shared component validators.
- `task.go`: task JSON envelope validation and command validation hooks.
- `observation.go`: observation JSON validation.
- `object.go`: object JSON validation by object type.
- `custom.go`: `custom_*` section validation.
- `command_schema.go`: restricted JSON Schema subset validation for command
  parameters.

## Public validator entry points

The initial implementation should expose resource-shaped entry points:

```go
func NormalizeEntity(entity *model.Entity) error
func NormalizeObject(obj *model.Object) error
func NormalizeTask(task *model.Task) error
func NormalizeObservation(obs *model.Observation) error
```

These functions mutate only the model's `JSON` field. They replace nil JSON
with `{}`, reject invalid input, and store canonical JSON bytes back on the
model before the function layer writes the resource.

## Common JSONB rules

Every JSON blob:

- must be valid JSON
- must be a JSON object
- must not be `null`
- must not be an array, string, number, or boolean
- must not duplicate promoted database fields at the top level
- must obey max byte size
- must obey max nesting depth
- must obey max key length
- must obey max field count
- may contain `custom_*` sections only within lightweight size, depth, key, and
  field-count limits

Promoted fields belong in normal columns, not caller-owned JSON. Top-level JSON
keys with these names must be rejected:

- `entity_id`
- `object_id`
- `task_id`
- `observation_id`
- `type`
- `status`
- `owner_type`
- `owner_id`
- `asset_id`
- `source_asset_id`
- `command_catalog_object_id`
- `created_at`
- `updated_at`
- `version`

The promoted-field rule is path-aware. It rejects top-level duplicates such as
`json.type`, but it must not reject valid nested domain fields such as
`json.components.command.type`.

## Entity validation

Entities share one validator and branch by `entity.Type`.

The expected JSON envelope is:

```json
{
  "components": {
    "telemetry": {},
    "status": {},
    "supported_commands": {}
  },
  "extra": {}
}
```

### Entity type rules

Asset entities:

- require `json.components.supported_commands`
- may have `telemetry`
- may have `heartbeat`
- may have `health`
- may have `communications`
- may have `status`
- may have `sensor_refs`
- may have `custom_*` components

Track entities:

- may have `telemetry`
- may have `status`
- may have `sensor_refs`
- may have `fusion_summary`
- may have `custom_*` components

Geofeature entities:

- require `json.components.geometry`
- may have `status`
- may have `custom_*` components

Unknown component names are rejected unless they use the `custom_*` prefix.

## Component validators

Vertical Slice 2 starts with validators for:

- `supported_commands`
- `telemetry`
- `status`
- `geometry`
- `heartbeat`
- `health`
- `communications`
- `sensor_refs`
- `fusion_summary`

Component validation should follow the atlas-c3 component contracts where they
are already defined. In particular, telemetry must enforce the established
constraints for:

- `observed_at`
- `latitude`
- `longitude`
- `altitude`
- `speed`
- `heading`

Latitude and longitude must stay in valid geographic ranges.

## Task validation

Tasks are validated in phases because full command validation touches the active
command catalog.

### Phase 2A

Validate the task JSON envelope:

- `json.components.command.type` is required
- `json.components.command.type` must be a non-empty string
- `json.components.parameters` is required
- `json.components.parameters` must be a JSON object
- promoted task fields must not be duplicated inside top-level JSON

### Phase 2B

Add command-aware validation:

- `asset_id` must reference an entity with `type = asset`
- the asset must have `json.components.supported_commands`
- the asset's `supported_commands.commands` must include the requested command
  type
- `command_catalog_object_id` must reference an object with
  `type = command_catalog`
- the command type must exist in the command catalog
- parameters must validate against the command's restricted JSON Schema subset

Current ATLAS2 stores `command_catalog_object_id` as a required task column.
Vertical Slice 2 validates against that pinned object. Active catalog
resolution can be a later slice unless command catalog materialization is
explicitly pulled into this one.

## Observation validation

Observation JSON must support incomplete sensing data. A bearing/elevation
observation without range is valid when represented as a line-of-bearing style
sighting; unknown range must be omitted or represented as `null`, not invented.

On create:

- `json.state` is required
- `json.state` must be `active`, `inactive`, or `ended`

On update/upsert:

- if `json.state` is present, it must be `active`, `inactive`, or `ended`
- if the chosen update contract requires fully shaped observation JSON, then
  `json.state` remains required there too

For all observation writes:

- `json.latest_sighting` is optional
- `json.latest_sighting` must be valid when present
- `json.sightings_object_id` is optional
- `json.extra` is allowed
- promoted observation fields must not be duplicated inside top-level JSON

## Object validation

Objects branch by `object.Type`:

- `command_catalog`
- `log`
- `photo`

Object JSON may contain type-specific payload or metadata. Relationship truth
stays in columns:

- `owner_type`
- `owner_id`

Do not duplicate ownership or resource identity inside object JSON.

The following object JSON keys are system-reserved because Vertical Slice 1 uses
them for the object manifest cache:

- `manifest`
- `manifest_version`

Caller-originated object create, update, and upsert payloads must not overwrite
reserved system-managed keys unless the call path is the internal manifest cache
writer.

## Custom sections

`custom_*` sections are allowed only as explicitly bounded extension points.
They must:

- be JSON objects
- use keys within the configured key-length limit
- stay within the custom section byte limit
- stay within the custom section depth limit
- stay within the custom section field-count limit

`custom_*` must not bypass core validation. If a field is a known core component
or known top-level section, it must validate through the core validator.

## Error behavior

All validators return field-path errors.

Examples:

- `json`
- `json.type`
- `json.components`
- `json.components.telemetry.latitude`
- `json.components.supported_commands.commands`
- `json.latest_sighting.observed_at`

The function layer should return these errors directly as invalid-input errors
before any store call happens.

## Testing scope

Tests should prove:

- invalid JSON bytes are rejected before store writes
- non-object JSON values are rejected
- nil JSON normalizes to `{}`
- promoted top-level fields inside JSON are rejected
- nested non-promoted fields such as `json.components.command.type` are allowed
- max size, depth, key length, and field count are enforced
- `custom_*` sections are accepted only within limits
- unknown entity components are rejected unless `custom_*`
- asset entities require `supported_commands`
- geofeatures require `geometry`
- telemetry latitude and longitude ranges are enforced
- task JSON requires `command.type`
- task JSON requires object-shaped `parameters`
- observation create requires valid `state`
- observation `latest_sighting` validates when present
- bearing/no-range sightings remain valid
- object JSON is validated by object type
- object JSON cannot overwrite reserved manifest cache keys
- every covered function write path calls blob validation before store writes

## Acceptance criteria

Vertical Slice 2 is complete when:

- invalid JSON object shapes are rejected before store writes
- promoted fields inside top-level JSON are rejected
- unknown entity components are rejected unless `custom_*`
- `custom_*` blobs have size, depth, key, and field limits
- asset entities require `supported_commands`
- geofeatures require `geometry`
- task JSON requires `command.type`
- observation create JSON requires valid `state`
- object JSON is validated by object type
- object manifest cache keys are protected from caller-originated writes
- all validators return field-path errors
- tests cover valid and invalid blobs for every resource family
