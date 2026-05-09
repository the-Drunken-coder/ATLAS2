# Atlas Core Vertical Slice 2: JSONB Validation and Normalization

## Purpose

Vertical Slice 2 makes Atlas Core's caller-owned JSONB blobs meaningful before
they reach persistence.

> No JSON blob reaches a store write unless the function layer validates and
> normalizes it.

Stores remain persistence-only. Stores do not interpret JSON meaning. The
function layer enforces resource semantics before calling stores.

## Validation modes

Validation has two layers.

### Pure JSON validation

Pure JSON validation:

- checks JSON syntax
- checks object shape
- checks promoted-field duplication
- checks size, nesting, key, and field-count limits
- checks component names
- checks resource-local JSON structure
- normalizes JSON bytes

Pure JSON validation has no store dependencies and lives in
`internal/blobvalidation`.

### Semantic cross-resource validation

Semantic cross-resource validation:

- checks asset existence and type
- checks asset-supported commands
- checks the pinned command catalog object exists (a `document` with `id =
  command_catalog`)
- checks command type existence in the pinned catalog
- checks command parameters against the pinned catalog schema

This validation lives in the function layer, or behind resolver interfaces
passed from the function layer. Basic JSON normalization must not depend on
Postgres or object storage.

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

## Related contract documents

This slice defines the validation system and write-path integration. The
supported JSON sections and component shapes are defined in:

- `docs/vertical-slice-2/component-contracts.md`

This file is primarily a **summary** of that contract, the `examples/` payloads,
and how validation plugs into the codebase. If anything here disagrees with
`component-contracts.md` or the `full*` / `minimum` examples under `examples/`,
treat those sources as authoritative for resource-local JSON shapes and correct
this summary. When the contract intentionally changes, update this spec,
`component-contracts.md`, and the examples together.

## Example JSON blobs

The examples folder contains valid JSON blob examples for each resource family.
Each file has a `minimum` (smallest contract-satisfying) example and one or more maximal `full*` variants (for example `full`, `full_success`, or `full_error`), depending on the resource examples provided:

- `docs/vertical-slice-2/examples/assets.json`
- `docs/vertical-slice-2/examples/tracks.json`
- `docs/vertical-slice-2/examples/geofeatures.json`
- `docs/vertical-slice-2/examples/tasks.json`
- `docs/vertical-slice-2/examples/observations.json`
- `docs/vertical-slice-2/examples/objects-log.json`
- `docs/vertical-slice-2/examples/objects-photo.json`
- `docs/vertical-slice-2/examples/objects-document.json`
- `docs/vertical-slice-2/examples/custom-sections.json`

These files are examples only. The authoritative required and optional field
rules live in `docs/vertical-slice-2/component-contracts.md`. Examples must
remain valid JSON and must not contain comments.

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

Exception: internal manifest-cache writes (`UpdateObjectManifest`) do not call
`NormalizeObject`. They must call
`internal/manifestvalidation.ValidateObjectManifest` before the store write.

If a write path is kept public, it must validate. If a write path should not be
supported, remove or hide it instead of leaving an unvalidated bypass.

## Validation errors at API boundaries

Blob validation is meant to fail with **specific, field-targeted errors**
(paths under `json.*`, messages callers can act on). That detail is useless if a
transport layer replaces it with a generic failure or hides it behind logging
only.

For any entry point that accepts caller-owned JSON (including future HTTP APIs):

- **Surface validation failures.** If normalization or validation returns an
  error, the boundary handler must not swallow it and return only "bad request",
  an empty body, or an undifferentiated internal error. Forward structured
  validator output into the client-visible response shape.
- **Separate categories.** Treat validation and normalization failures as their
  own client-facing category (for example HTTP 400 plus a structured error body),
  distinct from authentication, authorization, not-found, conflict, and true
  internal faults.
- **Preserve paths and messages.** Prefer exposing validator field paths and
  messages over paraphrasing them away; paraphrasing tends to lose the precision
  callers need to fix payloads.

True internal failures (dependency outages, invariant violations, bugs) stay out
of that category and must not be confused with JSON contract violations.

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
- `command_schema.go`: restricted JSON Schema subset primitives used by
  command parameter validation.

## Public validator entry points

The initial implementation should expose resource-shaped entry points:

```go
type Operation string

const (
	OperationCreate Operation = "create"
	OperationUpdate Operation = "update"
	OperationUpsert Operation = "upsert"
)

func NormalizeEntity(entity *model.Entity, op Operation) error
func NormalizeObject(obj *model.Object, op Operation) error
func NormalizeTask(task *model.Task, op Operation) error
func NormalizeObservation(obs *model.Observation, op Operation) error
```

These functions mutate only the model's `JSON` field. They replace nil JSON
with `{}`, reject invalid input, and store canonical JSON bytes back on the
model before the function layer writes the resource.

Operation context is required because create, update, and upsert can have
different required sections. For example, observation create requires
`json.state`; an update may be full-model replacement or patch-style depending
on the function contract.

## Update semantics

Current ATLAS2 function-layer update methods accept full resource models, not
named-section patch documents. For full-model writes, validators should enforce
the full resource contract for that operation.

If a future API introduces patch-style writes:

- only touched sections are validated first
- the patch is applied to the existing stored JSON
- the resulting full resource JSON is then validated before any store write

This applies to entity, object, task, and observation updates.

For full-model task writes, `json.components.command.type` and
`json.components.parameters` remain required. For patch-style task writes, a
progress-only patch can validate the touched progress section, but the resulting
stored task JSON must still satisfy the full task contract before persistence.

## Compatibility

Vertical Slice 2 validates new writes only. It does not backfill, reject, or
repair already-stored legacy rows during startup, list, or get operations. Read
paths may decode best-effort unless a later migration or repair slice introduces
strict at-rest validation.

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

Nil JSON normalizes to `{}`, then resource-specific validation still runs. For
example:

- an asset with nil JSON still fails create because `supported_commands` is
  missing
- a geofeature with nil JSON still fails create because `geometry` is missing
- a task with nil JSON still fails create because `command.type` and
  `parameters` are missing

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

The promoted-field rule is path-aware. Treat JSON paths as dot-separated
segments rooted at `json`. Reject a promoted field name only when it appears as
an immediate child of the root object, for example `json.type`. The same name
is allowed at deeper nesting levels, for example `json.type.nested_field`,
`json.components.type`, `json.components.command.type`, and
`json.extra.entity_id`.

Canonical JSON normalization means:

- nil becomes `{}`
- object keys are emitted in deterministic `encoding/json` map-key order
- output is not pretty-printed
- output has no trailing whitespace
- unknown allowed extension fields are preserved
- semantic values are not rewritten except where explicitly normalized

Normalization must be idempotent. Calling `NormalizeX` twice on the same valid
model must produce identical JSON bytes and no additional semantic changes.

Validation limits:

This section is the canonical source for Vertical Slice 2 numeric validation
limits. Related docs should reference these values instead of repeating them.

- max JSON blob size: 64 KiB
- max nesting depth: 16
- max total object fields: 500
- max key length: 100 characters
- max `custom_*` section size: 16 KiB
- max `custom_*` nesting depth: 8
- max `custom_*` key length: 100 characters
- max `custom_*` total fields: 100

Unknown top-level keys are rejected unless they are explicitly allowed for that
resource or use the `custom_*` prefix. Unknown data belongs under `extra` or a
bounded `custom_*` section, not arbitrary top-level keys.

Allowed top-level entity JSON keys:

- `components`
- `extra`
- `custom_*`

Allowed top-level task JSON keys:

- `description`
- `created_by`
- `components`
- `extra`
- `custom_*`

Allowed top-level observation JSON keys:

- `state`
- `latest_sighting`
- `sightings_object_id`
- `extra`
- `custom_*`

Allowed top-level object JSON keys depend on `object.Type`. All object types
also allow:

- `extra`
- `custom_*`

The promoted-field ban only applies to top-level fields. Valid nested
promoted-like domain fields include:

- `json.components.command.type`
- `json.latest_sighting.kind`
- `json.extra.type`
- `json.custom_vendor.type`

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

- require `json.components.telemetry` with both `latitude` and `longitude`
  (the track's best-estimate position; an optional
  `telemetry.uncertainty_radius_m` describes the horizontal uncertainty around
  that estimate, typically supplied by the data fusion system)
- may have `status`
- may have `fusion_summary` (the home for detection-provenance metadata,
  pointing at heavier provenance via `provenance_object_id`)
- may have `custom_*` components

Track JSON describes the track itself (what it is, where it is, our posture
toward it). Detection-side metadata (which sensors observed it, fusion process
detail) belongs in `fusion_summary` and the referenced provenance object, not
in track components.

Geofeature entities:

- require `json.components.geometry`
- may have `status`
- may have `custom_*` components

Unknown component names are rejected unless they use the `custom_*` prefix.

For entity JSON:

- `components` is required when required components are checked
- missing `components` normalizes to `{}` only when the entity type has no
  required components for that operation
- `extra` is optional and normalizes to `{}` when omitted
- `components` and `extra` must be objects when present

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
- `altitude_m`
- `speed_m_s`
- `heading_deg`

Latitude and longitude must stay in valid geographic ranges.

## Task validation

Tasks are validated in phases because full command validation touches the pinned
command catalog object.

### Phase 2A

Validate the task JSON envelope:

- `json.components.command.type` is required
- `json.components.command.type` must be a non-empty string
- `json.components.parameters` is required
- `json.components.parameters` must be a JSON object
- unknown keys inside `json.components` are rejected; names with the `custom_*`
  prefix belong at the top level of task JSON (alongside `components` and
  `extra`), not inside `components`
- promoted task fields must not be duplicated inside top-level JSON

This phase belongs in `internal/blobvalidation` and has no store dependencies.

### Phase 2B

Add command-aware validation:

- `asset_id` must reference an entity with `type = asset`
- the asset must have `json.components.supported_commands`
- the asset's `supported_commands.commands` must include the requested command
  type
- `command_catalog_object_id` must reference the object with
  `id = command_catalog` (a `document` object holding the catalog JSON)
- the command type must exist in the command catalog
- parameters must validate against the command's restricted JSON Schema subset

This phase belongs in the function layer, not in pure JSON normalization. The
function layer may implement the checks directly or pass resolver interfaces
into a semantic validator, for example:

```go
type TaskValidationResolver interface {
	GetAsset(ctx context.Context, assetID string) (*model.Entity, error)
	GetCommandCatalog(ctx context.Context, objectID string) (*CommandCatalog, error)
}

func ValidateTaskCommandSemantics(ctx context.Context, task *model.Task, resolver TaskValidationResolver) error
```

Current ATLAS2 stores `command_catalog_object_id` as a required task column.
Vertical Slice 2 validates against that pinned object. Active catalog
resolution can be a later slice unless command catalog materialization is
explicitly pulled into this one.

Phase 2B is complete when the function layer can validate task parameters
against a pinned command catalog object if a catalog resolver is available. If
command catalog materialization is not implemented in this slice,
`command_schema.go` may include only the restricted JSON Schema validator
primitives and tests, with runtime catalog loading deferred.

For task JSON:

- `components` is required for full-model writes because command fields live
  under it
- `extra` is optional and normalizes to `{}` when omitted
- `components` and `extra` must be objects when present

## Observation validation

Observation JSON must support incomplete sensing data. A bearing/elevation
observation without range is valid when represented as a line-of-bearing style
sighting; unknown range must be omitted or represented as `null`, not invented.

On create:

- `json.state` is required
- `json.state` must be `active`, `inactive`, or `ended`

On full-model update/upsert:

- `json.state` is required
- `json.state` must be `active`, `inactive`, or `ended`

On future patch-style update:

- if `json.state` is touched, it must be `active`, `inactive`, or `ended`
- after applying the patch, the resulting full observation JSON must still
  contain valid `json.state`

For all observation writes:

- `json.latest_sighting` is optional
- `json.latest_sighting` must be valid when present
- `json.sightings_object_id` is optional
- `json.extra` is allowed
- promoted observation fields must not be duplicated inside top-level JSON

Slice 2 validates the `latest_sighting` envelope only:

```json
{
  "observed_at": "2026-01-01T00:00:10Z",
  "kind": "line_of_bearing",
  "data": {},
  "extra": {}
}
```

Minimum rules:

- `observed_at` is required and must be RFC 3339
- `kind` is required and must be a non-empty string
- `data` is required and must be an object
- `extra` is optional and must be an object when present

Kind-specific sighting validation is deferred except that for `line_of_bearing`
sightings, a range/distance measurement is optional: a sighting may include
bearing and elevation without a known distance.

## Object validation

Objects branch by `object.Type`:

- `log`
- `photo`
- `document`

The command catalog is stored as a `document` object with `id = command_catalog`
and a JSON payload; there is no separate `command_catalog` object type.

Object JSON may contain type-specific payload or metadata. Relationship truth
stays in columns:

- `owner_type`
- `owner_id`

Do not duplicate ownership or resource identity inside object JSON.

Minimum object JSON shapes:

`log` object JSON:

- `log_type` is optional and must be a string when present
- `started_at` is optional and must be RFC 3339 when present
- `ended_at` is optional and must be RFC 3339 when present
- `extra` is optional and must be an object when present

`photo` object JSON:

- `content_type` is optional and must be a string when present
- `captured_at` is optional and must be RFC 3339 when present
- `width_px` is optional and must be a positive integer when present
- `height_px` is optional and must be a positive integer when present
- `extra` is optional and must be an object when present

`document` object JSON:

- `content_type` is optional and must be a string when present
- `extra` is optional and must be an object when present
- the document payload (e.g. JSON, markdown, XML) lives in object files, not
  `object.json`
- the command catalog is stored as a `document` object with
  `id = command_catalog`; there is no separate `command_catalog` object type

The following object JSON keys are system-reserved because Vertical Slice 1 uses
them for the object manifest cache:

- `manifest`
- `manifest_version`

Caller-originated object create, update, and upsert payloads must not overwrite
reserved system-managed keys unless the call path is the internal manifest cache
writer.

The internal manifest cache update path, `UpdateObjectManifest`, is the only
writer allowed to set these keys. `NormalizeObject` must reject caller-supplied
`manifest` and `manifest_version`, and `UpdateObjectManifest` must not call
`NormalizeObject`. `UpdateObjectManifest` must call
`internal/manifestvalidation.ValidateObjectManifest`, and that validator must
succeed before the manifest cache write reaches the store.

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

All validators return field-path errors. Validation errors should have a stable
shape:

```go
type Violation struct {
	Field   string
	Code    string
	Message string
}

type ValidationError struct {
	Violations []Violation
}
```

If the implementation reuses `model.FieldError` for single-field failures, it
must preserve the same `field`, `code`, and `message` information. Multi-field
validation should still expose individual field paths.

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
- nil JSON normalizes to `{}` before resource-specific required-field checks
- promoted top-level fields inside JSON are rejected
- nested non-promoted fields such as `json.components.command.type` are allowed
- nested promoted-like fields such as `json.latest_sighting.kind`,
  `json.extra.type`, and `json.custom_vendor.type` are allowed
- max size, depth, key length, and field count are enforced
- normalization is idempotent for every resource family
- `custom_*` sections are accepted only within limits
- unknown entity components are rejected unless `custom_*`
- unknown top-level keys are rejected unless allowed or `custom_*`
- asset entities require `supported_commands`
- track entities require `telemetry.latitude` and `telemetry.longitude`
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

Use fake stores that record whether `Create`, `Update`, or `Upsert` was called.
For invalid JSON, assert the function returns a validation error and the fake
store call count is zero.

## Acceptance criteria

Vertical Slice 2 is complete when:

- invalid JSON object shapes are rejected before store writes
- promoted fields inside top-level JSON are rejected
- operation context is passed to every resource normalizer
- pure JSON validation is separated from cross-resource semantic validation
- top-level allowed-key rules prevent arbitrary JSON junk
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
