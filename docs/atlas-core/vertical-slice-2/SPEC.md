# Atlas Core Vertical Slice 2: Protocol Integration

> **Historical document.** Vertical-slice numbering is no longer the primary
> planning axis; use [design-decisions/](../design-decisions/) (ADRs 0001–0005)
> and the current `atlas-core/services/...` layout for navigation. Do not invest
> in a full rewrite of this spec; fix only actively misleading sections below.

## Status

Implemented on `main` (historical milestone label).

Atlas Protocol now has a first local baseline: resource contracts, schemas,
valid examples, invalid goldens, TypeScript and Go validators, change-event
protocol coverage, stable validation issue shape, and `atlas.py protocol-check`
verification.

Vertical Slice 2 is the Atlas Core work that consumes that protocol baseline.
It must not redefine Atlas JSON document structure inside Core.

Current `main` includes the Core protocol-validation adapter, function-layer
validation before persistence, task runtime checks, and focused tests for the
adapter, no-write behavior, and task runtime behavior.

## Goal

Atlas Core should validate caller-owned Atlas JSON with Atlas Protocol before
persistence, then apply Core-owned runtime checks that require stored system
state.

In short:

> Atlas Protocol answers, "Is this well-formed Atlas-shaped data?"
> Atlas Core answers, "Is this write valid for the current stored system?"

## Source Documents

Protocol source of truth:

- `docs/atlas-protocol/README.md`
- `docs/atlas-protocol/contracts/resources.md`
- `docs/atlas-protocol/contracts/command-catalog.md`
- `docs/atlas-protocol/contracts/change-events.md`
- `docs/atlas-protocol/contracts/errors.md`
- `docs/atlas-protocol/conformance.md`
- `atlas-protocol/source/schemas/`
- `atlas-protocol/source/manifests/`
- `atlas-protocol/source/goldens/invalid/`
- `atlas-protocol/packages/go/`

Core implementation context:

- `atlas-core/services/functions/internal/function/`
- `atlas-core/services/shared/store/`
- `atlas-core/services/datastorage/internal/postgres/`
- `atlas-core/services/shared/model/`
- `atlas-core/services/functions/cmd/atlas-functions/`

## Non-Goals

This slice must not:

- add a Core-local schema system
- duplicate protocol schemas or custom protocol rules
- introduce database migrations
- implement command execution
- implement data fusion
- implement public HTTP API behavior beyond whatever already exists in Core
- publish npm or Go protocol packages
- add new protocol package targets
- implement durable change-event delivery, SSE, Postgres notifications, or an
  outbox table

Change-event **documents** are protocol-ready and validated in this slice.
**Delivery:** a best-effort `SubscribeMutations` gRPC stream on `atlas-functions`
exists (see ADR 0002); it is not a durable or resumable event log and is out of
scope for full protocol change-event pipeline work described in atlas-protocol.
Product client sync (SDK stream + strict full list sync) is described in
[plans/plan.md](../plans/plan.md).

## Ownership Boundaries

### Atlas Protocol Owns

- entity JSON shape
- task JSON shape
- observation JSON shape
- object JSON shape
- command catalog JSON shape
- change-event document shape
- validation issue shape: `field`, `code`, `message`
- custom protocol rules that do not require stored Core state
- valid examples and invalid golden cases
- TypeScript/Go validator parity

### Atlas Core Owns

- choosing when to validate before persistence
- mapping model rows to protocol resource kinds and variants
- protecting store boundaries
- checking current-state runtime semantics
- mapping protocol issues into Core-facing errors without losing protocol issue
  data
- proving invalid writes do not reach stores

## Integration Architecture

Add a narrow protocol-validation adapter in Atlas Core. Suggested package:

```text
atlas-core/services/shared/protocolvalidation/
```

Responsibilities:

- import the local Go protocol package from `atlas-protocol/packages/go`
- construct the protocol validator once and reuse it
- expose Core-shaped validation methods for function-layer callers
- translate Core model types into protocol resource kind and variant
- return protocol issues without rewriting `field`, `code`, or `message`

Suggested interface:

```go
type Validator interface {
    ValidateEntity(entity *model.Entity) []protocol.ValidationIssue
    ValidateObject(obj *model.Object) []protocol.ValidationIssue
    ValidateTask(task *model.Task) []protocol.ValidationIssue
    ValidateObservation(obs *model.Observation) []protocol.ValidationIssue
}
```

Implementation notes:

- `model.Entity.Type` maps to protocol resource `entity` with variant
  `asset`, `track`, or `geofeature`.
- `model.Object.Type` maps to protocol resource `object` with variant `log`,
  `photo`, or `document`, except `command_catalog`.
- `model.ObjectTypeCommandCatalog` is not a normal object JSON variant.
  Its `JSON` must validate as protocol resource `commandCatalog`.
- `model.Task.JSON` maps to protocol resource `task`.
- `model.Observation.JSON` maps to protocol resource `observation`.
- Change events are not validated from Core write paths in this slice because
  Core is not producing events yet.

## Function-Layer Placement

Protocol validation belongs in `atlas-core/services/functions/internal/function`,
before any store write.

Store interfaces in `atlas-core/services/shared/store` and implementations in
`atlas-core/services/datastorage/internal/postgres` should remain persistence-oriented. They should
not import the protocol validator and should not become responsible for protocol
shape rules.

Validation should run before:

- `EntityFunctions.CreateEntity`
- `EntityFunctions.UpdateEntity`
- `EntityFunctions.UpsertEntity`
- `ObjectFunctions.CreateObject`
- `ObjectFunctions.UpdateObject`
- `ObjectFunctions.UpsertObject`
- `TaskFunctions.CreateTask`
- `TaskFunctions.UpdateTask`
- `TaskFunctions.UpsertTask`
- `ObservationFunctions.CreateObservation`
- `ObservationFunctions.UpdateObservation`
- `ObservationFunctions.UpsertObservation`

Delete paths do not validate caller-owned JSON because they receive identifiers,
not resource documents.

## Runtime Checks

Runtime checks stay in Core because they require stored state.

Task writes should eventually check:

- target asset exists
- target entity is an asset
- target asset supports the task command
- command catalog object exists
- command catalog object validates as protocol `commandCatalog`
- requested command exists in the stored catalog
- task parameters match the stored catalog's `parameters_schema`

These checks should run in the function layer after static protocol validation
and before persistence.

Do not add ad hoc protocol-shape checks for these. If a check can be expressed
without stored state, prefer adding it to Atlas Protocol and its shared
conformance cases.

## Error Mapping

Atlas Protocol validation issues are canonical:

```json
{
  "field": "json.components.command.type",
  "code": "required",
  "message": "command.type is required"
}
```

Core may wrap protocol validation failure in a Core error type, but it must
preserve the issue array with `field`, `code`, and `message` unchanged.

Implementation options:

- add a new Core error type that carries `[]protocol.ValidationIssue`
- or add a typed validation error in `internal/protocolvalidation` and map it at
  API boundaries later

Avoid flattening multiple protocol issues into one string-only `FieldError`.
That would lose the conformance shape that Atlas Protocol guarantees.

Suggested stable Core code:

```text
VALIDATION_FAILED
```

## Implementation Steps

### 1. Wire The Go Protocol Package

Add a local dependency from `atlas-core` to `atlas-protocol/packages/go`.

Expected shape during local development:

```text
require atlas.local/protocol v0.0.0
replace atlas.local/protocol => ../atlas-protocol/packages/go
```

Do not publish a module as part of this slice.

### 2. Add The Core Adapter

Create `atlas-core/services/shared/protocolvalidation` (implemented there today).

The adapter should:

- initialize the embedded protocol validator with `protocol.New()`
- validate raw `JSON` fields from Core models
- select protocol resource and variant from promoted model fields
- expose focused methods for the function layer
- return exact protocol issues

### 3. Inject Validation Into Function Constructors

Update function constructors to accept the protocol validator dependency.

Keep the dependency explicit. Avoid package globals.

For tests, use either:

- the real protocol validator for integration-style tests
- a small fake validator for tests that only prove ordering and store calls

### 4. Validate Before Store Writes

Each create/update/upsert path should validate:

- model-level required fields and promoted fields
- protocol JSON shape
- Core runtime checks, where applicable

The write must stop before calling the store if protocol validation fails.

### 5. Add Runtime Task Checks

After static protocol task validation:

- load the target asset
- verify it is an asset
- load the command catalog object referenced by the task
- validate the command catalog JSON through Atlas Protocol
- check the task command exists in the catalog
- check parameters against the catalog's lightweight `parameters_schema`

Keep parameter matching in Core because it depends on the currently stored
catalog selected by the task.

### 6. Preserve Store Boundaries

Do not add protocol validation to:

- `atlas-core/services/shared/store`
- `atlas-core/services/datastorage/internal/postgres`
- SQL/schema setup

Stores may still enforce database constraints and persistence invariants, but
they should not become the protocol source of truth.

### 7. Tests

Add focused tests proving:

- invalid entity JSON does not reach `EntityStore`
- invalid object JSON does not reach `ObjectStore`
- invalid command catalog JSON does not reach `ObjectStore`
- invalid task JSON does not reach `TaskStore`
- invalid observation JSON does not reach `ObservationStore`
- protocol issues preserve `field`, `code`, and `message`
- task runtime checks reject missing target asset
- task runtime checks reject unsupported command
- task runtime checks reject missing command catalog
- task runtime checks reject command parameters that do not match the stored
  catalog

Use fake stores for no-write assertions where practical. Use Postgres-backed
tests only when proving integration with real persistence behavior.

## Completion Criteria

Vertical Slice 2 is complete when:

- Core validates entity, object, command catalog, task, and observation JSON
  through Atlas Protocol before persistence
- Core keeps protocol validation out of store and Postgres packages
- Core runtime checks remain in the function layer
- protocol validation failures preserve exact protocol issue objects
- invalid protocol writes do not reach stores
- invalid runtime writes do not reach stores
- local and CI verification cover both protocol and Core integration behavior

## Verification

Run:

```bash
python3 atlas.py protocol-check
cd atlas-core && go test -p 1 ./...
git diff --check
```

Targeted tests should include focused `go test -run` filters for:

- protocol-validation adapter behavior
- function-layer no-write behavior
- task runtime checks

If Postgres is unavailable, Postgres-backed tests fail by default (see
`testsupport.RequirePostgresOrSkip` in `AGENTS.md`). Set
`ATLAS_SKIP_POSTGRES_TESTS=true` only when intentionally running without a database.
Function-layer tests with fake stores should not depend on Postgres.

## Open Questions Before Implementation

These can be answered while implementing, but they should be made explicit in
the PR:

- What exact Core error type should carry multiple protocol issues?
- Should command catalog parameter matching initially support only required
  presence and primitive type checks, or also reject unknown task parameters?
- Should Core validate command catalog JSON only when catalog objects are
  written, or also every time a task references a stored catalog?
