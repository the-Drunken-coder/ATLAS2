# Atlas Core Vertical Slice 3: Public API Foundation

> **Historical document.** Vertical-slice numbering is no longer the primary
> planning axis; use [design-decisions/](../design-decisions/) (ADRs 0001–0005)
> and the current `atlas-core/services/...` layout for navigation. Do not invest
> in a full rewrite of this spec; fix only actively misleading sections below.

## Status

Planning draft.

Vertical Slice 1 established Atlas Core storage, stores, function-layer
operations, local runtime, and service startup.

Vertical Slice 2 is implemented on `main`: Atlas Core validates caller-owned
JSON through Atlas Protocol in the function layer before persistence and applies
Core-owned runtime checks where stored state is required.

Vertical Slice 3 should design the Atlas SDK and expose the already-built Core
behavior through a **public HTTP JSON API** that supports that package.

Architecture (service boundaries, API entrypoints): [ADR 0002](../design-decisions/0002-service-boundaries-grpc-changefeed.md).
HTTP idempotency at the product edge: [ADR 0001](../design-decisions/0001-api-boundary-idempotency-versioning.md).

## Goal

Atlas Core should provide a small, stable HTTP surface and a TypeScript SDK that
lets clients create, read, update, list, delete, and subscribe to the core
resources already supported by the function layer.

In short:

> Atlas Core functions own behavior (internal gRPC).
> The Atlas SDK owns the client-facing developer experience.
> The HTTP API is the bridge between that package and the Core function layer.

## Source Documents

Core foundation:

- `docs/atlas-core/vertical-slice-1/SPEC.md`
- `docs/atlas-core/vertical-slice-2/SPEC.md`
- `docs/atlas-core/design-decisions/0001-api-boundary-idempotency-versioning.md`
- `docs/atlas-sdk/README.md`

Protocol source of truth:

- `docs/atlas-protocol/README.md`
- `docs/atlas-protocol/contracts/resources.md`
- `docs/atlas-protocol/contracts/command-catalog.md`
- `docs/atlas-protocol/contracts/errors.md`
- `atlas-protocol/source/schemas/`
- `atlas-protocol/source/goldens/`

Core implementation context:

- `atlas-core/services/functions/cmd/atlas-functions/`
- `atlas-core/services/functions/internal/function/`
- `atlas-core/services/shared/model/`
- `atlas-core/services/shared/store/`
- `atlas-core/services/datastorage/internal/postgres/`
- `atlas-core/services/datastorage/internal/objectstorage/`

Legacy reference:

- legacy Atlas `atlas-api-helper-npm` package (external reference)

## Non-Goals

This slice must not:

- move validation or runtime checks out of the function layer
- let API handlers call store or Postgres implementations directly
- duplicate Atlas Protocol schemas in API code
- implement command execution
- implement data fusion
- implement streaming RPC, Postgres notifications, or an outbox table
- fully design authentication or authorization
- add database migrations
- expose internal idempotency keys as part of the public API
- build Go, Rust, Python, or multi-language connection packages
- publish npm packages externally

SDK sync is in scope at the API and SDK boundary. It includes server-filtered
subscriptions, service events, local caches, and refresh. Durable event replay,
cross-process delivery, and exactly-once delivery are not in scope for this
slice.

Assets should use narrow subscription-backed caches (required) to receive only
the data relevant to them. Broad current-state sync may be used as an optional
fallback when narrow scopes are not feasible.

## Design Direction

Design the Atlas SDK first, then use that package shape to drive the API.

The package should be the primary consumer-facing interface. The API should not
be designed as a disconnected collection of routes and then wrapped afterward.
Instead, define the client methods that Atlas applications naturally want, then
make the server API a clean bridge to the Core function layer.

This deliberately builds both sides before finalizing the bridge:

- the Core side already has stores, functions, protocol validation, and runtime
  checks
- the client side should have a clear TypeScript package shape
- the HTTP API should connect those two islands with the least awkward contract

## Atlas SDK

Vertical Slice 3 should include a TypeScript-only Atlas SDK.

Do not plan Go, Rust, Python, or other language clients for this system unless a
future requirement creates a real need. A single npm package is sufficient for
the current system and avoids the parity burden from the previous Atlas
connection packages.

The package should be usable from modern Node and browser-like runtimes that
provide `fetch`, with an option to inject a custom fetch implementation.

At a high level, the package should own:

- base URL configuration
- optional authentication token/header behavior
- service health and readiness calls
- entity methods
- object metadata methods
- object file methods
- task methods
- observation methods
- sync helpers for server-filtered subscriptions, service events, local cache,
  and refresh
- typed API errors
- request/response TypeScript types

The package should not own:

- Core business rules
- Atlas Protocol schema definitions
- persistence behavior
- command execution behavior
- hidden retries that change mutation semantics

The legacy `atlas-api-helper-npm` package is a useful reference for the shape of
the developer experience: one client object, central request handling, optional
token support, injectable `fetch`, resource methods, object helpers, query
helpers, and typed errors. The new package should reuse that lesson, not copy
the old API one-for-one.

## Transport Direction

Start with HTTP JSON.

This is the simplest useful public boundary for local development, smoke tests,
the TypeScript SDK, and future integration work. A later slice can add
ConnectRPC if a worker, relay, or synchronization use case requires it.

Suggested package (not created yet):

```text
atlas-core/services/api/
```

Suggested responsibilities:

- own HTTP routing
- decode request bodies
- validate transport-level request shape
- map requests to `atlas-functions` gRPC (not stores or datastorage directly)
- map function-layer errors to HTTP responses
- encode public response DTOs
- expose readiness and liveness endpoints
- expose SDK sync behavior for server-filtered subscriptions, service events,
  local cache refresh, and event delivery

The API package should not:

- import `services/datastorage/internal/postgres`
- import `services/datastorage/internal/objectstorage` directly
- perform Atlas Protocol validation directly
- contain Core runtime tasking rules

## API Boundary Model

Use public DTOs at the HTTP boundary.

Do not serialize `internal/model` structs directly. Current model types are
database/function-layer shapes and may include fields, tags, or versioning
details that are not the right public contract.

Boundary mapping should make these choices explicit:

- which fields are accepted from clients
- which fields are server-owned
- whether update operations are full replacement or partial update
- whether raw row `version` is hidden, exposed through ETags, or not exposed
  initially
- how Atlas Protocol validation issues are represented in error responses

The existing API boundary decision says client-visible idempotency keys are not
part of the public API. Handlers should not accept idempotency keys from headers
or request bodies, and should not pass caller-provided keys into function-layer
options.

## Initial Resource Scope

Vertical Slice 3 should expose the resources already supported by Core:

- service status
- SDK sync
- entities
- objects
- object files
- tasks
- observations

The first API pass should prioritize a coherent SDK surface over specialized
workflow endpoints.

### Health And Readiness

Minimum endpoints:

- liveness: process is up
- readiness: Core has initialized storage, schema, object storage, validation,
  and reconciled object state

The existing ready-file healthcheck can remain for Docker. The HTTP readiness
endpoint should report the same service-readiness concept through the API.

### Sync

Minimum operations:

- subscribe to a narrow scope (required: clients must use narrow
  subscription-backed caches to receive only relevant data)
- subscribe to broad current-state scope (optional fallback: only when narrow
  scopes are not feasible)
- receive service events for subscribed scopes
- refresh subscribed scopes
- remove subscriptions

The first version should provide a simple way for the TypeScript SDK to observe
successful Core resource changes for the resources or views the caller has
subscribed to. It should be enough for local clients, assets, and development
tools to react to changes without polling every resource.

Subscriptions should be server-filtered from the start. Systems that subscribe
to task views, entity views, or other constrained-link data should not receive
unrelated Core events as the normal operating model.

Keep the event system intentionally modest in this slice. Do not require durable
replay, cross-process delivery, database notifications, or exactly-once
semantics yet.

The API/SDK bridge should make these subscriptions stable for application code.
The implementation can evolve inside the API, bridge, and SDK without requiring
each asset or command interface to rewrite its data-access code.

### Entities

Minimum operations:

- create entity
- get entity
- list entities
- update entity
- delete entity

Entity writes should call `EntityFunctions`. API code should rely on the
function layer for Atlas Protocol validation.

### Objects

Minimum operations:

- create object
- get object info
- list objects
- update object info
- delete object
- get object content
- put object content

Object info should return object metadata, object JSON, and manifest/file
metadata in one response, but not file bytes.

Object content endpoints should make the common one-file object case easy while
still allowing a caller to select a file for multi-file objects.

Object JSON writes and object file operations should call `ObjectFunctions`.
Command catalog objects remain ordinary object writes at the HTTP boundary, but
the function layer must continue validating their JSON as protocol
`commandCatalog`.

The API should preserve filesystem-safety behavior by going through
`ObjectFunctions`. Handlers should not construct object-storage paths.

### Tasks

Minimum operations:

- create task
- get task
- list tasks
- update task
- delete task

Task writes should call `TaskFunctions`, preserving current runtime checks for
target assets, supported commands, command catalogs, and command parameters.

### Observations

Minimum operations:

- create observation
- get observation
  - by ID
  - by time or time window
  - by source asset
  - by related track or target, once that relationship is explicit in the data
    model
- list observations
- update observation
- delete observation

Observation writes should call `ObservationFunctions`. The API should not invent
a separate observation-specific protocol path outside the existing model and
function layer.

Observation reads may become high-volume. The API/SDK bridge should avoid
forcing clients to fetch broad observation lists and filter locally when callers
need a specific time window, source asset, track, or target.

## Error Contract

The API should use a consistent JSON error envelope.

At minimum, errors should include:

- stable error code
- human-readable message
- optional details
- optional protocol validation issues

Protocol validation failures must preserve the Atlas Protocol issue array with:

- `field`
- `code`
- `message`

The API can wrap those issues in an HTTP error envelope, but it must not flatten
or rewrite them into a lossy string.

Suggested status mapping:

- malformed JSON request body: `400 Bad Request`
- invalid request shape: `400 Bad Request`
- Atlas Protocol validation failure: `422 Unprocessable Entity`
- Core runtime validation failure: `422 Unprocessable Entity`
- missing resource: `404 Not Found`
- version/precondition conflict, if implemented: `409 Conflict` or
  `412 Precondition Failed`
- unexpected internal failure: `500 Internal Server Error`

## Version And Concurrency

The public API must make an explicit choice before implementing update routes.

Options:

- use `ETag` and `If-Match`, mapping internally to model `Version`
- accept last-write-wins at the HTTP boundary for the first API slice

Do not accidentally expose raw internal `version` fields through direct model
serialization.

If the first implementation chooses last-write-wins, document that decision in
this slice and keep the handler behavior intentional.

## App Integration

`app.New()` should initialize the API server only after Core dependencies are
ready:

- config
- logger
- Postgres pool
- schema
- object storage
- protocol validator
- function layer
- startup reconciliation

Shutdown should stop the API server cleanly before closing backing resources.

Configuration should include:

- API listen address
- API read timeout
- API write timeout
- API shutdown timeout
- authentication enabled/disabled

Use conservative defaults for local development.

Authentication should be treated as a designed system that can be enabled for
real deployments and disabled easily for development. The first API planning
slice should leave room for that boundary without making authentication a hard
requirement for local development.

## Tests

Add focused tests at the API boundary:

- route registration and method handling
- malformed JSON handling
- request DTO decoding
- response DTO encoding
- error mapping for protocol validation failures
- error mapping for runtime validation failures
- not-found responses
- object file read/write behavior through functions
- readiness/liveness responses
- SDK sync behavior, including server-filtered subscriptions and service events

Use fake function implementations where practical so handler tests stay fast and
do not require Postgres.

Add at least one integration smoke test that proves the real app can start the
API server when its dependencies are available.

## Completion Criteria

Vertical Slice 3 is complete when:

- Atlas Core starts an HTTP JSON API server
- a TypeScript Atlas SDK exists as the primary client interface
- API handlers call the function layer rather than stores directly
- public DTOs are used instead of direct `internal/model` serialization
- CRUD endpoints exist for entities, objects, tasks, and observations
- object file endpoints go through `ObjectFunctions`
- readiness and liveness endpoints exist
- SDK sync exists for initial asset/task/entity use cases and is consumable
  from the SDK
- protocol validation issues survive API error mapping unchanged
- runtime validation failures map to stable API errors
- client-provided idempotency keys are not accepted as public API behavior
- update version/concurrency behavior is explicitly documented
- local tests verify API behavior without requiring every handler test to use
  Postgres
- SDK tests verify the public client methods against mocked transport behavior

## Verification

Run:

```bash
cd atlas-core && go test -p 1 ./...
git diff --check
```

If API smoke tests require Postgres, they should use `testsupport.RequirePostgresOrSkip`
and fail when a test database is unavailable unless `ATLAS_SKIP_POSTGRES_TESTS=true`.

## Open Questions Before Implementation

- Should the first API implementation use last-write-wins updates, or should it
  implement `ETag` and `If-Match` immediately?
- What exact public JSON envelope should success responses use: bare resources
  or `{ "data": ... }` wrappers?
- Should list endpoints support pagination in the first pass, or return the
  current full list behavior exposed by stores?
- What request size limits should apply to JSON bodies and object file uploads?
- Should object files be uploaded as raw request bodies first, or multipart
  form uploads?
- What should the first service event payload include?
- Should the SDK expose raw route-shaped methods only, or also small convenience
  methods where they clearly improve client ergonomics?
