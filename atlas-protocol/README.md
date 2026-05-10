# Atlas Protocol

Atlas Protocol is the local, TypeScript-first source of truth for Atlas caller-owned JSON document shapes.

## Status

This package is local-only for now. It is intentionally kept inside the ATLAS2 checkout so Atlas Core can validate caller-owned JSON against the same schemas and custom Atlas protocol rules that external tooling would use later.

Publishing is deferred. Atlas Core development should use the local package and synced runtime artifacts only.

## What lives here

- `schemas/`: JSON Schema draft 2020-12 source-of-truth documents.
- `examples/`: protocol examples copied from Vertical Slice 2 plus protocol-only examples.
- `src/`: TypeScript runtime API, custom Atlas rules, error normalization, and CLI.
- `generated/`: generated TypeScript types, schema bundle, and standalone Ajv validators.
- `dist/`: runnable validator artifact for Atlas Core and local tools.

## Local workflow

```bash
cd atlas-protocol
corepack pnpm install
corepack pnpm run verify
```

`verify` regenerates protocol artifacts, rebuilds the validator CLI, and runs protocol tests.

Atlas Core should not call these commands directly. Use `python3 atlas.py protocol-sync` or `python3 atlas.py start`, which build Atlas Protocol and sync the runtime artifacts into `atlas-core/protocol/` before Atlas Core runs.

## Atlas Core boundary

Atlas Protocol owns:

- caller-owned JSON document shapes
- Atlas custom protocol rules that do not require live Atlas Core state
- protocol examples
- normalized protocol validation errors shaped as `{ field, code, message }`

Atlas Core still owns:

- Postgres schema and promoted columns
- object storage behavior and manifests
- cross-resource semantic checks
- command catalog existence and parameter validation against the stored catalog
- authorization and runtime state transitions

## Node and pnpm requirement

Node.js >= 20.11.0 and pnpm (via `corepack pnpm`) are required for local Atlas Protocol development. Atlas Core's local workflow makes missing tooling fail fast before the stack starts.
