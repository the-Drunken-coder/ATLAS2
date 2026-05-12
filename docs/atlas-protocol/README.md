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
4. [Conformance and local testing](conformance.md)
5. [Roadmap](roadmap.md)

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
- future change event shape
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

The current docs are a protocol seed, not a finished package. They establish:

- initial resource contracts
- an initial command catalog shape from the earlier Atlas project
- validation error shape
- local conformance and versioning policy
- deferred change-event questions

The next milestone is making these contracts machine-checkable without making
any package publish part of the local development loop.
