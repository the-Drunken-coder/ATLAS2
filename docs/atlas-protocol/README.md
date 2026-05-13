# Atlas Protocol

Atlas Protocol is the shared data contract for Atlas.

It defines what Atlas-shaped documents and messages mean, what makes them
valid, and which parts of that contract are reusable across Atlas Core, tools,
agents, clients, simulators, and future services.

Atlas Protocol is not Atlas Core, a database schema, an API transport, or a
package format. Those systems consume or publish protocol-shaped data.

## Reading Order

Start here:

1. [Protocol Boundary](#protocol-boundary)
2. [Contracts](contracts/README.md)
3. [Examples](examples/README.md)
4. [Conformance and local testing](conformance.md) (includes [Versioning Policy](conformance.md#versioning-policy))
5. [Roadmap](roadmap.md)

## How Atlas Core will use this later

Atlas Core will treat protocol validation as a **static shape** gate on stored
JSON: if a document fails protocol validation, it is not well-formed Atlas data.
Core then applies **runtime** checks (existence of assets, command catalog in
storage, authorization, supported commands, manifest cache rules, and so on).
Those runtime rules stay out of this repository’s protocol package; see the
Atlas Core integration docs for current runtime responsibilities.

## Protocol Boundary

Atlas Protocol answers:

> Is this valid Atlas-shaped data?

Atlas Core answers:

> Is this valid for the current stored system state?

Examples:

- Missing `task.components.command.type` is a protocol validation problem.
- A task targeting an asset that does not exist is an Atlas Core runtime
  problem.
- A malformed observation document is a protocol validation problem.
- A write blocked by authorization is an implementation/runtime problem.

## What Protocol Owns

Atlas Protocol owns the reusable contract:

- resource document shapes
- command catalog document shape
- validation error shape
- change event shape
- extension rules
- structural field constraints
- valid and invalid examples
- conformance behavior shared by package outputs

## What Implementations Own

Atlas Core and other implementations own runtime facts and side effects:

- storage and indexing
- promoted database columns
- object files and manifests
- idempotency behavior
- current-state lookups
- command catalog lookup in stored data
- asset existence and supported-command checks
- authorization
- write ordering and rollback behavior
- service startup and deployment

## Current Status

Atlas Protocol today combines human-readable contracts with machine-checkable
artifacts:

- JSON Schemas under `atlas-protocol/source/schemas/`
- a reference TypeScript validator in `atlas-protocol/packages/typescript/`
- a Go package target in `atlas-protocol/packages/go/`
- valid examples in `atlas-protocol/examples/` and the manifest
  `atlas-protocol/source/manifests/valid-examples.json`
- invalid golden payloads under `atlas-protocol/source/goldens/invalid/` and the
  manifest `atlas-protocol/source/manifests/invalid-cases.json`
- local `verify` / `validate` scripts (no package publish required for local
  checks; see [Conformance and local testing](conformance.md))

Resource contracts, command catalog shape, validation error shape, change event
shape, package conformance, and versioning policy are in place.
