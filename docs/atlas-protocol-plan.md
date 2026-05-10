# Atlas Protocol Extraction Plan

## Purpose

Atlas Protocol defines what valid Atlas JSON messages and documents look like.
Atlas Core is one implementation that stores, indexes, validates, and serves
those messages.

The goal is to move reusable protocol structure out of Atlas Core without
forcing Atlas Core to give up its own storage model. Atlas Core may continue to
promote fields into Postgres columns, persist caller-owned JSON as JSONB, build
indexes, and enforce cross-resource semantics in the function layer.

## Working Assumptions

- JSON remains the runtime message/document format.
- The source-of-truth protocol definitions should live outside `atlas-core/`.
- The first developer package should be TypeScript-first.
- The protocol should be usable locally before any public package is published.
- Atlas Core should consume the local protocol package during development.
- For the first implementation, assume `atlas-protocol/` is present in the
  ATLAS2 checkout whenever Atlas Core is run through `atlas.py`.
- Node and pnpm may be runtime/developer requirements for Atlas Core during
  this phase if that keeps validation easy, seamless, and always current.
- `atlas.py` should own the local update/build/sync flow so developers do not
  need to remember separate protocol commands before running Atlas Core.
- No in-message protocol version fields are planned for now.
- Package or repository releases may still be used later so external consumers
  can pin a known protocol state.
- Go and Rust bindings are not planned. Atlas Core should use Atlas Protocol's
  TypeScript/Ajv validation path instead of introducing a second Go schema
  validator.

## Non-Goals

- Do not add database migrations.
- Do not replace Atlas Core's function-layer semantic validation with schema
  validation.
- Do not keep duplicate Atlas Core JSON-shape validators after protocol
  validation is integrated and parity-tested.
- Do not publish a public package before the local system is functional.
- Do not introduce protocol version fields into every message as speculative
  compatibility machinery.
- Do not validate or repair old stored JSONB rows; local data is disposable
  during this development phase.

## Target Shape

```text
ATLAS2/
  atlas-protocol/
    package.json
    README.md
    schemas/
      entity.schema.json
      object.schema.json
      task.schema.json
      observation.schema.json
      change-event.schema.json
      validation-error.schema.json
      command-catalog.schema.json
    examples/
      assets.json
      tracks.json
      geofeatures.json
      objects-document.json
      objects-log.json
      objects-photo.json
      tasks.json
      observations.json
      custom-sections.json
      change-event.json
      validation-error.json
      command-catalog.json
    src/
      index.ts
      schemas.ts
      validate.ts
      atlas-rules.ts
      errors.ts
      types.ts
      cli.ts
    scripts/
      export-schema-bundle.ts
      generate-types.ts
      generate-standalone-validators.ts
    generated/
      types.ts
      validators/
        index.mjs
    dist/
      atlas-protocol-validator.mjs

  atlas-core/
    internal/protocolvalidation/
      validator.go
      validator_test.go
```

`atlas-protocol/schemas` is the language-neutral source of truth.
`atlas-protocol/src` is the TypeScript developer package around those schemas.
`atlas-protocol/generated` contains generated TypeScript types and compiled Ajv
validators derived from the schemas. `atlas-core/internal/protocolvalidation`
is the Go adapter that invokes the local protocol validator and maps its errors
into Core's validation error shape.

The protocol validator is not only a raw Ajv wrapper. It runs:

1. Ajv JSON Schema validation.
2. Atlas-specific protocol rules that are awkward or too domain-specific for
   readable JSON Schema.
3. Normalized Atlas validation errors.

## Boundary Rules

Protocol owns:

- resource JSON document shapes
- object JSON document shapes
- task JSON document shapes
- observation JSON document shapes
- command catalog JSON shape
- change event JSON shape
- validation error JSON shape
- local field constraints that do not require Atlas Core state
- structural validation rules that can be expressed without Atlas Core state
- Atlas-specific protocol validation rules that do not require live Atlas Core
  state, including:
  - promoted top-level fields are forbidden in caller-owned JSON blobs
  - `custom_*` immediate child keys cannot duplicate promoted fields, known
    top-level sections, or known component names
  - entity type determines allowed and required components
  - track telemetry requires latitude and longitude together
  - task JSON has the required command/parameters envelope shape
  - object type determines allowed object JSON fields
  - object `manifest` and `manifest_version` keys are system-reserved
  - command catalog document JSON has a keyed command map with
    `parameters_schema` per command
- canonical validation error shape for protocol/schema failures:
  `{ "field": "...", "code": "...", "message": "..." }`
- examples that prove the schemas are understandable and valid. Existing
  Vertical Slice 2 examples should preserve their current grouped-file style:
  each file has a `minimum` example and one or more `full*` examples such as
  `full`, `full_success`, or `full_error`.

Atlas Core owns:

- Postgres schema and promoted columns
- JSONB persistence layout
- object storage layout
- function-layer write-path validation
- cross-resource validation
- task command support checks
- command catalog object existence, catalog pinning, and command lookup against
  the currently stored catalog
- object ownership checks
- object manifest filesystem/cache behavior
- state transition rules
- authorization when that exists
- persistence-time serialization/canonicalization that is not protocol shape
  validation

Shared rule:

- Atlas Protocol is the single source of truth for caller-owned JSON document
  shapes. Atlas Core must not keep a second hand-authored shape validator for
  the same JSON.
- Protocol validation rejects structurally invalid documents before store
  writes.
- Function-layer semantic validation is still required before store writes.

## Locked Implementation Decisions

- Protocol schema dialect: JSON Schema draft 2020-12. Each schema file should
  include `$schema` with value
  `https://json-schema.org/draft/2020-12/schema`.
- Command parameter schemas use the restricted subset already implemented in
  Atlas Core's current `command_schema.go`: `type`, `properties`, `required`,
  `additionalProperties`, `items`, and `enum`, with scalar types `object`,
  `array`, `string`, `number`, `integer`, and `boolean`.
- Package layout: `atlas-protocol/` starts as a standalone pnpm package, not a
  repo-wide pnpm workspace.
- TypeScript validator: Ajv 8.
- TypeScript types: generated from JSON Schema, not hand-authored as the source
  of truth.
- Validation runtime: Atlas Protocol's TypeScript/Ajv path is the primary
  validation implementation. Atlas Core should not add a separate Go JSON Schema
  validator. Atlas Protocol validation includes both Ajv schema validation and
  custom Atlas protocol rules.
- Local consumption: `atlas.py` may assume the sibling `atlas-protocol/`
  directory exists in the same ATLAS2 checkout and should use that local package
  directly.
- Old stored data: no compatibility or repair requirement.
- New writes: caller-owned JSON validates through Atlas Protocol before
  persistence.
- Error mapping: protocol validation errors normalize to `field`, `code`, and
  `message` so Atlas Core can return the same field-targeted shape it returns
  today.

## Chunk 1: Protocol Package Skeleton

### Goal

Create `atlas-protocol/` as a local TypeScript-first package that can be used
inside the repo without publishing.

### Scope

- Add `atlas-protocol/package.json`.
- Add package scripts for schema validation, generated types, standalone Ajv
  validator generation, tests, and schema bundle export.
- Use standalone package wiring under `atlas-protocol/`; do not add a repo-wide
  pnpm workspace in the first pass.
- Add `README.md` explaining local-only status and future publishing intent.
- Add minimal `src/index.ts` exports.
- Add a CLI entry point that can validate JSON by schema name.

### Investigation Questions

- What exact package scripts should `atlas.py`, CI, and developers call?
- Should generated TypeScript files be checked in, generated during package
  build, or both?
- Should the first validator CLI validate from raw schemas, generated standalone
  Ajv validators, or support both modes?

### Completion Criteria

- `atlas-protocol` installs locally.
- The package can export at least one schema through TypeScript.
- The package can run `pnpm test`, generate types, and build a validator CLI.
- No publishing configuration is required to use it locally.
- Atlas Core is not touched in this chunk.

### Suggested Agent Prompt

Inspect the current ATLAS2 repo and propose the smallest `atlas-protocol`
TypeScript package skeleton that supports local development only. Use a
standalone pnpm package under `atlas-protocol/`; do not add repo-wide pnpm
workspace wiring yet. Return exact files to create, scripts, and validation
commands, but do not implement Atlas Core integration.

## Chunk 2: Initial Schema Inventory

### Goal

Translate the existing Vertical Slice 2 contract docs and examples into an
initial protocol schema inventory.

### Scope

- Add JSON Schemas for:
  - entity
  - object
  - task
  - observation
  - command catalog
  - validation error
  - change event
- Move or copy existing examples into `atlas-protocol/examples`.
- Preserve the existing grouped example keys (`minimum` and any
  resource-specific `full*` variants).
- Keep examples valid JSON with no comments.
- Preserve current `custom_*` extension rules where practical.
- Inventory Atlas-specific protocol rules that should live in
  `atlas-protocol/src/atlas-rules.ts` rather than in JSON Schema or Atlas Core.

### Investigation Questions

- Which current rules in `docs/vertical-slice-2/component-contracts.md` map
  directly to JSON Schema?
- Which current rules are semantic or stateful and must remain in Atlas Core
  function-layer validation instead of protocol schemas?
- How should schemas express `custom_*` top-level extension slots?
- Which `custom_*`, promoted-field, entity-type, object-type, and task-envelope
  checks should be implemented as Atlas custom rules instead of schema keywords?
- Should each resource family be one schema with discriminators, or multiple
  focused schemas such as `entity.asset.schema.json`?

### Completion Criteria

- Every existing VS2 example has a corresponding protocol example.
- The task success and error examples remain distinct rather than being
  collapsed into a single `full` task example.
- Protocol examples validate against protocol schemas.
- Any rule that cannot be represented cleanly in JSON Schema is listed as
  Atlas custom protocol validation or Core semantic validation.
- No Atlas Core behavior changes yet.

### Suggested Agent Prompt

Read `docs/vertical-slice-2/component-contracts.md`,
`docs/vertical-slice-2/SPEC.md`, and all files under
`docs/vertical-slice-2/examples`. Produce an inventory that separates rules into
JSON-Schema-expressible, Atlas custom protocol validation, and Core semantic
validation. Treat Core-only validation as live-system semantics, not a second
JSON-shape validator. Then outline the exact initial schema files, custom rule
files, and example files needed under `atlas-protocol/`.

## Chunk 3: TypeScript Runtime API

### Goal

Expose a useful TypeScript API around the schemas without making TypeScript the
only protocol truth.

### Scope

- Export raw schema objects.
- Export validation helpers.
- Export TypeScript types derived from the schemas.
- Provide structured validation errors that preserve field paths and messages.
- Use Ajv 8 for validation.
- Add `atlas-rules.ts` for custom Atlas protocol rules that Ajv should not own.
- Add `errors.ts` for deterministic `{ field, code, message }` error output.
- Generate TypeScript types from JSON Schema.
- Generate standalone Ajv validator modules when practical so runtime validation
  does not need to compile schemas on every invocation.
- Add tests that prove examples validate and malformed examples fail.

### Investigation Questions

- What error shape should the TypeScript package expose so it can map cleanly to
  Atlas Core validation errors later?
- How should Ajv errors and Atlas custom-rule errors be merged and sorted?
- Which generator should create TypeScript types from JSON Schema?
- Should the CLI read JSON from files only, stdin only, or both?
- Should the CLI support one-shot validation and a long-running stdin/stdout
  worker mode for Atlas Core?

### Completion Criteria

- TypeScript consumers can import schemas, validators, and types.
- Tests cover every protocol example.
- Generated types are reproducible from schemas.
- Error output is deterministic enough for tests and docs.
- Ajv failures and Atlas custom-rule failures both return the same normalized
  error shape.
- Package runtime is independent of Atlas Core and can be used directly by
  Atlas Core through a CLI/worker adapter.

### Suggested Agent Prompt

Design the TypeScript public API for `atlas-protocol`. Include raw schema
exports, validator helpers, generated types, custom Atlas rule execution, and
normalized error output. Recommend the smallest API that supports local
agents/tools today and future package publishing. Include example imports and
test strategy.

## Chunk 4: Validator and Schema Bundle Export

### Goal

Create stable artifacts that Atlas Core and non-TypeScript systems can consume
from the local package or from a future published package.

### Scope

- Add a build/export script that writes a single schema bundle JSON file.
- Add a build script that writes a runnable validator artifact, preferably an
  Ajv standalone-generated module wrapped by a small CLI.
- Include schema identifiers, schema contents, and package metadata useful for
  debugging.
- Keep the bundle deterministic.
- Check generated artifacts into `atlas-protocol/generated` or `dist` only if
  that is the easiest way to keep `atlas.py`, CI, and Atlas Core runtime
  seamless.

Initial schema bundle shape:

```json
{
  "name": "atlas-protocol",
  "schema_dialect": "https://json-schema.org/draft/2020-12/schema",
  "schemas": [
    {
      "name": "task",
      "path": "schemas/task.schema.json",
      "sha256": "...",
      "schema": {}
    }
  ]
}
```

The bundle must not include generated timestamps. Sort schemas by `name`.

### Investigation Questions

- Should the bundle include examples, or only schemas?
- Should the export script validate all schemas before writing the bundle?
- Does Ajv standalone generation cover the schema features used by the initial
  protocol schemas?
- Is one-shot CLI validation enough, or should Atlas Core use a long-running
  validator worker to avoid process startup per write?

### Completion Criteria

- Running one package script creates a deterministic schema bundle.
- The bundle contains every schema Atlas Core needs.
- Running one package script creates a validator artifact.
- The validator artifact can validate example JSON without TypeScript source
  files.
- The bundle can later come from either local path or a published package.

### Suggested Agent Prompt

Design the `atlas-protocol` schema bundle and validator artifacts. Specify the
JSON shape, export command, generated validator command, and whether examples
belong in the bundle. Assume Node is an acceptable Atlas Core runtime
dependency if it keeps validation single-sourced.

## Chunk 5: `atlas.py` Protocol Workflow

### Goal

Make protocol setup and freshness automatic through the existing local stack
entrypoint.

### Scope

- Update `atlas.py` so starting or testing the system can install protocol
  package dependencies when needed.
- Assume `atlas-protocol/` is available locally in the ATLAS2 checkout; fail
  clearly if that folder is missing.
- Add commands or startup checks that build protocol schemas, generated types,
  validator artifacts, and bundles.
- Fail fast if Node or pnpm is missing.
- Make stale generated protocol artifacts visible and hard to ignore.
- Keep the workflow simple enough that normal local development does not require
  memorizing protocol-specific commands.

### Investigation Questions

- Should `atlas.py` always rebuild protocol artifacts on start, or only check
  freshness and tell the developer what command to run?
- What is the exact repo-relative path contract between `atlas.py`,
  `atlas-protocol/`, and `atlas-core/`?
- Should Docker images include Node and pnpm, or should local-only workflows use
  host Node while containers use prebuilt artifacts?
- What files should CI compare to prove generated artifacts are current?

### Completion Criteria

- `atlas.py` makes a missing or stale protocol package obvious before Atlas Core
  runs.
- Running Atlas Core through `atlas.py` always uses the local `atlas-protocol/`
  package, not a published package.
- One command can rebuild protocol artifacts and run relevant checks.
- CI has a freshness check for generated types, bundles, and validator
  artifacts.
- No store or Postgres schema changes are required.

### Suggested Agent Prompt

Inspect `atlas.py`, Docker/Compose files, and current CI workflows. Propose the
smallest protocol setup/update workflow that makes Atlas Protocol validation
easy, seamless, and resilient. Node and pnpm may be required. Identify exact
commands, failure modes, and freshness checks.

## Chunk 6: Atlas Core Protocol Validation Adapter

### Goal

Replace Atlas Core's duplicated JSON-shape validation with Atlas Protocol
validation while keeping Core-owned semantic checks in Core.

### Scope

- Add `atlas-core/internal/protocolvalidation`.
- Implement a Go adapter that invokes the Atlas Protocol validator CLI or
  worker.
- Map Atlas Protocol validation errors into Core's field-targeted error shape.
- Replace `internal/validation/blob.NormalizeEntity`, `NormalizeObject`,
  `NormalizeTask`, and `NormalizeObservation` call sites with protocol
  validation calls.
- Delete or retire the duplicated Go JSON-shape validation once parity tests
  pass.
- Keep Core semantic validation in place for resource existence, command
  support, command catalog object existence, command lookup against the current
  catalog, ownership, manifest filesystem/cache behavior, and DB-column
  validation.

### Investigation Questions

- Which current `internal/validation/blob` tests should move to
  `atlas-protocol`, and which should remain in Atlas Core as semantic tests?
- Which current checks in `internal/validation/blob` map to JSON Schema, which
  map to `atlas-rules.ts`, and which are actually Core semantics?
- Should Atlas Core call a one-shot CLI per write, maintain a long-running
  validator worker process, or validate at a higher function boundary where
  process startup cost is less important?
- What timeout, stderr, and crash behavior should the adapter use?
- Which normalization behavior currently done by `NormalizeX` must be preserved
  in Atlas Protocol?

### Completion Criteria

- New entity, object, task, and observation writes validate caller-owned JSON
  through Atlas Protocol before persistence.
- Core semantic checks still run before store writes.
- Old Go shape validators are removed or left only as temporary test fixtures
  with a deletion TODO and no production call sites.
- Parity tests prove the current VS2 examples pass and representative invalid
  payloads fail with field-targeted errors, including invalid cases currently
  covered by Atlas Core's custom blob validators.

### Suggested Agent Prompt

Inspect `atlas-core/internal/validation/blob`, `atlas-core/internal/service`,
and `atlas-core/internal/validation/manifest`. Design the replacement path from
Go blob validators to Atlas Protocol validation. Keep Core semantic validation
in Core, move Atlas-specific JSON-shape rules into `atlas-protocol`, remove
duplicated JSON-shape validation from production paths, and identify the tests
needed before deletion.

## Chunk 7: Change Event Protocol

### Goal

Define the change event message shape before adding a live fan-out system.

### Scope

- Turn the open changefeed hook question into protocol-level event shape
  options.
- Decide minimal vs rich event payload for the initial protocol.
- Define event fields for resource kind, operation, resource id, timestamp, and
  optional metadata.
- Keep implementation of SSE, ConnectRPC, LISTEN/NOTIFY, or outbox deferred.

### Investigation Questions

- Should events carry only identifiers, or include post-state snapshots?
- How should object manifest updates be represented? The current Slice 1
  changefeed note already resolves that a `MANIFEST_CACHE_SYNC_ERROR` partial
  failure must not emit a change event; only a successful function path or a
  later successful reconciler path may publish one.
- How should idempotent replay be represented, if at all?
- What event shape serves both future REST/SSE and internal worker streams?

### Completion Criteria

- `change-event.schema.json` exists.
- Examples cover create, update, delete, and manifest/cache-related events if in
  scope.
- Manifest cache partial-failure behavior is documented as non-emitting.
- Deferred delivery mechanisms remain explicitly deferred.
- Atlas Core does not emit events until a later implementation chunk.

### Suggested Agent Prompt

Read `docs/vertical-slice-1/CHANGEFEED-HOOK.md` and current function-layer
mutation methods. Recommend the first `change-event.schema.json` shape for
Atlas Protocol. Keep the event schema useful for SSE and internal streams, but
do not design the delivery implementation.

## Chunk 8: Documentation and Governance

### Goal

Make the protocol understandable and changeable without forcing every consumer
to rediscover the architecture.

### Scope

- Add `atlas-protocol/README.md`.
- Document local development workflow.
- Document how Atlas Core invokes the local protocol validator and uses the
  schema bundle for debugging/freshness checks.
- Document what changes require updating examples.
- Document package publishing as future work.
- Optionally add an ADR for extracting Atlas Protocol from Atlas Core.

### Investigation Questions

- Should this be captured as an ADR under `docs/design-decisions/`?
- What is the minimum governance process before publishing?
- How should breaking changes be handled if there are no in-message versions?
- How should downstream consumers learn that their local bundle is stale?

### Completion Criteria

- A new contributor can understand protocol vs core boundaries.
- Local edit/build/test loop is documented.
- Publishing is explicitly deferred.
- The Node/pnpm requirement is documented clearly, and no public npm publish is
  required for local Atlas Core development.

### Suggested Agent Prompt

Draft the docs/governance material for Atlas Protocol extraction. Focus on the
boundary between protocol and Atlas Core, local-only development before
publishing, no in-message versioning, and how future consumers should pin or
sync protocol changes.

## Recommended Implementation Order

These chunks can be investigated in parallel, but implementation should land in
an order that keeps the repo buildable:

1. Chunk 8: ADR/docs decision, if the team wants the architecture accepted first.
2. Chunk 1: package skeleton.
3. Chunk 2: initial schema inventory and schema files.
4. Chunk 3: TypeScript runtime API.
5. Chunk 4: validator and schema bundle export.
6. Chunk 5: `atlas.py` protocol workflow.
7. Chunk 6: Atlas Core protocol validation adapter and blob-validator removal.
8. Chunk 7: change event protocol, unless changefeed work becomes urgent first.

## Parallel Work Matrix

| Chunk | Can Run In Parallel With | Blocks |
| --- | --- | --- |
| 1. Package skeleton | 2, 7, 8 | 3, 4 |
| 2. Schema inventory | 1, 7, 8 | 3, 4, 6 |
| 3. TypeScript API | 5, 7, 8 after 1 and 2 | external TS consumers |
| 4. Bundle export | 3, 5 after 1 and 2 | 5, 6 |
| 5. `atlas.py` workflow | 3, 4, 7, 8 | 6 |
| 6. Core adapter and validator removal | 7, 8 after 3, 4, and 5 | completing protocol-owned validation |
| 7. Change event protocol | 1, 2, 3, 8 | future changefeed implementation |
| 8. Docs/governance | all chunks | publishing |

## Key Risks

- JSON Schema may not express every current validator rule cleanly.
- Keeping duplicated rules between schemas and Go validators can create drift;
  the production path should converge on Atlas Protocol validation.
- Requiring Node and pnpm makes setup larger; `atlas.py`, Docker, and CI must
  fail clearly when those tools are missing.
- Per-write subprocess validation may be simple but could become slow or
  fragile; a long-running validator worker may be needed if write volume or
  process startup cost becomes annoying.
- Avoiding in-message versioning is simpler now but requires discipline around
  package pins, Git tags, and consumer upgrade timing later.
- A broad validator rewrite could destabilize Vertical Slice 2; delete
  handwritten validators only after parity tests prove the protocol validator
  covers the same caller-owned JSON behavior.

## First Milestone

The first useful milestone is intentionally modest:

- `atlas-protocol/` exists locally.
- It exports JSON Schemas and TypeScript validation helpers.
- It uses Ajv 8 and JSON Schema draft 2020-12.
- It generates TypeScript types from JSON Schema.
- Current VS2 examples have protocol equivalents.
- Protocol examples validate in TypeScript tests.
- It can build a deterministic schema bundle and runnable validator artifact.
- Atlas Core is unchanged except, optionally, documentation and `atlas.py`
  checks that name the future validation adapter.

This milestone proves the protocol package can stand on its own before it is
wired into Atlas Core.

## Second Milestone

The second milestone replaces duplicate production validation:

- `atlas.py` installs/checks/builds Atlas Protocol artifacts before Atlas Core
  runs.
- Atlas Core invokes Atlas Protocol validation for new caller-owned JSON writes.
- Atlas Protocol validation includes Ajv schema validation and Atlas custom
  protocol rules.
- Core semantic checks still run after protocol shape validation.
- Production call sites no longer use the old Go blob shape validators.
- Parity tests cover the current VS2 valid examples and representative invalid
  payloads.
