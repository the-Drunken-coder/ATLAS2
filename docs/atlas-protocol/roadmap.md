# Atlas Protocol Roadmap

## Direction

Build Atlas Protocol first. Atlas Core Vertical Slice 2 should consume the
protocol after the protocol contract is clear enough to implement.

The durable goal is:

> Define Atlas data once, then let Atlas Core, tools, clients, agents, and
> future services use the same definition.

## Current Seed Material

- [Resource contracts](contracts/resources.md)
- [Command catalog](contracts/command-catalog.md)
- [Validation errors](contracts/errors.md)
- [Examples](examples/README.md)
- [Conformance](conformance.md)

## Stage 1: Stabilize The Human Contract

Goal: make the protocol understandable before building package machinery.

Work:

- tighten resource contract language
- settle command catalog shape
- keep change events explicitly deferred
- identify protocol-owned rules versus Core-owned runtime rules

Done when:

- a new contributor can explain what belongs in protocol
- the examples match the contract docs
- no Core integration doc duplicates protocol shape rules

## Stage 2: Add Machine-Checkable Contracts

Goal: make the human contract checkable.

Work:

- choose the first schema/contract representation
- encode resource and command catalog contracts
- define valid and invalid golden cases
- define stable validation error output

Done when:

- valid examples can be checked automatically
- invalid examples produce expected `field` and `code` errors
- local checks do not require package publishing

## Stage 3: Build The First Package Target

Goal: prove one ecosystem-native package can consume the shared protocol source.

Work:

- create the first package target
- expose validation primitives
- run shared conformance tests
- generate local artifacts for tests

Done when:

- one package target validates the shared examples and golden cases
- the package can be used locally without publishing
- generated outputs are reproducible

## Stage 4: Integrate Atlas Core

Goal: make Atlas Core consume Atlas Protocol without making Core the source of
truth for protocol shape.

Work:

- add a narrow Core integration point
- validate caller-owned data before store writes
- keep runtime/state checks in Core
- map protocol errors into Core errors

Done when:

- Core writes use protocol validation
- Core semantic checks still run in Core
- Vertical Slice 2 is an integration spec, not a second protocol spec

## Stage 5: Add Package Targets When Needed

Goal: publish ecosystem-native outputs from one protocol source.

Work:

- add TypeScript, Python, Go, CLI, or bundle outputs only when needed
- keep all package outputs on the same Atlas Protocol version
- run the same conformance suite for every output

Done when:

- each published package exposes the same protocol version behavior
- local testing still does not require publishing

## Open Questions

- Should command `parameters_schema` remain the lightweight Atlas shape or be
  translated to full JSON Schema?
- Which invalid golden cases are required for the first build?
- What is the first package target?
- How much of change events should be defined before a delivery mechanism
  exists?
