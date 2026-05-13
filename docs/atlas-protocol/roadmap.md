# Atlas Protocol Roadmap

## Direction

Build Atlas Protocol first. Atlas Core should consume the protocol after the
protocol contract is clear enough to implement.

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
- settle the first change event document shape
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

### Milestone status for first delivery

- Stages 1–3 are complete enough for the first Protocol merge baseline: human
  contract, machine-checkable contract, first package target, and reproducible
  local checks.
- Stage 4 is the next Atlas Core consumer/integration phase.
- Stage 5 now has TypeScript and Go package targets; add further targets only
  when a consumer needs them.

## Stage 4: Integrate Atlas Core

Goal: make Atlas Core consume Atlas Protocol without making Core the source of
truth for protocol shape.

Work:

- add a narrow Core integration point
- validate caller-owned data before store writes
- keep runtime/state checks in Core
- map protocol errors into Core errors
- defer detailed Atlas Core implementation responsibilities to the Core docs so
  protocol docs do not become a second implementation plan

Done when:

- Core writes use protocol validation
- Core semantic checks still run in Core
- Atlas Core docs remain implementation-focused instead of becoming a second
  protocol spec

## Stage 5: Add Package Targets When Needed

Goal: publish ecosystem-native outputs from one protocol source.

Work:

- add package targets only when needed
- keep all package outputs on the same Atlas Protocol version
- run the same conformance suite for every output

Done when:

- each published package exposes the same protocol version behavior
- local testing still does not require publishing

## Decisions (current)

- First machine package target is the local TypeScript validator in
  `atlas-protocol/packages/typescript/`.
- First Go package target is the local module in `atlas-protocol/packages/go/`.
- Invalid golden coverage is represented by
  `atlas-protocol/source/goldens/invalid/` with
  `atlas-protocol/source/manifests/invalid-cases.json`.
- Command `parameters_schema` remains the lightweight object map to
  `parameterDef` values in
  `atlas-protocol/source/schemas/command-catalog.schema.json`, not arbitrary
  embedded JSON Schema payloads.
- Change events use row-plus-json snapshots for `created` and `updated`; deleted
  events carry `snapshot: null`. Delivery remains implementation-owned.
