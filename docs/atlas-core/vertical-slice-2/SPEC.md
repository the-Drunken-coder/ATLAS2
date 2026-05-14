# Atlas Core Vertical Slice 2: Protocol Integration

## Status

Deferred until Atlas Protocol completion is accepted.

Vertical Slice 2 should not define Atlas JSON document structure directly.
Those contracts now live in Atlas Protocol:

- `docs/atlas-protocol/contracts/resources.md`
- `docs/atlas-protocol/examples/`

## Purpose

Vertical Slice 2 is reserved for integrating Atlas Core with Atlas Protocol
once the protocol contract is clear enough to consume.

The protocol-completion phase does not require Atlas Core code changes. It
should make Protocol ready for Core integration by proving TypeScript and Go
validator parity, change-event protocol validation, stable protocol issue
shape, and local/CI verification.

The intended direction is:

> Atlas Protocol defines valid Atlas-shaped data. Atlas Core validates
> caller-owned data against that protocol before persistence, then applies
> Core-owned runtime semantics.

## Deferred To Atlas Protocol

Atlas Protocol owns the reusable data contract for:

- entity JSON
- task JSON
- observation JSON
- object JSON
- command catalog JSON
- validation error shape
- change event shape
- valid and invalid examples
- extension rules
- protocol-level field constraints

Atlas Core must not duplicate those protocol rules as its own separate source
of truth.

## Atlas Core Responsibilities

When this slice is reopened, Atlas Core should focus on implementation behavior
that only Core can own:

- validating caller-owned JSON before store writes
- preserving store-layer persistence-only boundaries
- keeping cross-resource semantic checks in the function layer
- checking asset existence and type
- checking asset-supported commands
- checking command catalog object existence and lookup
- validating command parameters against the currently stored catalog
- protecting object manifest/cache fields from caller writes
- mapping protocol validation failures into Core errors
- proving invalid writes do not reach stores

## Current Non-Scope

Until Atlas Protocol has a stable first contract, this slice should not choose:

- a Core-local JSON schema implementation
- a permanent validator package layout
- a language runtime bridge
- generated SDKs
- public HTTP API behavior
- data fusion behavior
- command execution behavior
- database migrations

## Reopen Criteria

Reopen Vertical Slice 2 when:

- Atlas Protocol has a first resource contract.
- Protocol examples exist for the document families Core must write.
- Protocol validation errors have a stable field-targeted shape.
- There is a clear integration path for Core to consume protocol validation.

At that point, this file should become an Atlas Core integration spec, not a
second copy of the protocol contract.
