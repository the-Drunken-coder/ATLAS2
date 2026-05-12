# Conformance And Local Testing

## Purpose

Atlas Protocol should be implementable in multiple languages without each
package drifting into a different contract.

Conformance tests are the shared proof that each package exposes the same
protocol behavior.

## Source Of Truth

The protocol source of truth is:

- human-readable protocol docs
- machine-checkable schemas or equivalent contract definitions
- custom protocol rule specs
- valid examples
- invalid golden cases
- expected normalized validation errors

Generated package types are outputs, not the source of truth.

## Custom Protocol Rules

Custom protocol rules are validation rules that are part of Atlas Protocol but
are awkward, unreadable, or unavailable in the base schema language.

Examples:

- promoted fields are forbidden only at the top level
- `custom_*` sections are allowed but bounded
- entity type controls allowed and required components
- track telemetry requires latitude and longitude together
- command catalog command IDs must be unique
- command catalog entries must use `parameters_schema`, not
  `parameter_schema`

These are different from Atlas Core runtime rules.

Atlas Core runtime rules require stored state, such as:

- target asset exists
- target asset supports the command
- stored command catalog exists
- task parameters match the currently stored catalog
- object manifest cache behavior is valid

## Golden Tests

Golden tests should be language-neutral fixtures that every package target can
run.

Suggested structure:

```text
atlas-protocol/
  examples/
  golden/
    valid-cases.json
    invalid-cases.json
```

Each valid case should include:

- case name
- document family
- input JSON
- expected normalized JSON when normalization is part of the contract

Each invalid case should include:

- case name
- document family
- input JSON
- expected errors with at least `field` and `code`

Example invalid case:

```json
{
  "name": "track_missing_longitude",
  "family": "entity",
  "context": {
    "entity_type": "track"
  },
  "input": {
    "components": {
      "telemetry": {
        "latitude": 40.7
      }
    }
  },
  "errors": [
    {
      "field": "json.components.telemetry",
      "code": "INVALID_VALUE"
    }
  ]
}
```

## Local Test Workflow

Local development should not require publishing packages.

The intended loop is:

```text
edit protocol docs/contracts/examples/rules
run local protocol check
build local artifacts/packages
run package conformance tests
run Atlas Core integration tests when Core consumes the protocol
```

Publishing is not part of the normal test loop.

Every package target should be able to consume local protocol source or local
generated artifacts during tests.

## Build Versus Publish

Build is local and frequent:

- validates schemas or equivalent contract definitions
- checks valid examples
- checks invalid golden cases
- generates local package artifacts
- runs package tests

Publish is deliberate and release-driven:

- happens on tagged protocol releases
- publishes packages/artifacts for ready ecosystems
- uses the same protocol version for every package output

## Versioning Policy

All package outputs share the Atlas Protocol version.

If the protocol is `0.2.0`, then the package outputs for that release should
also be `0.2.0`:

- npm package: `0.2.0`
- Python package: `0.2.0`
- Go module tag: `v0.2.0`
- CLI artifact: `0.2.0`
- schema/protocol bundle: `0.2.0`

The number describes the Atlas Protocol system, not the maturity of one
language package.

Not every package target has to exist for every release. If Python is not ready
for `0.2.0`, do not publish a Python package for that release. But any package
that is published for `0.2.0` must expose protocol `0.2.0` behavior.

## Package Outputs

The long-term model is one protocol source with many ecosystem-native outputs:

```text
source protocol
  docs
  schemas
  rules
  examples
  golden tests
outputs
  TypeScript package
  Python package
  Go module
  CLI binary
  schema/protocol bundle
  maybe WASM or C ABI later
```

TypeScript can be the first package target, but it should not become the center
of the protocol universe.

## First Milestone

The first milestone should prove:

- the resource contracts are documented
- command catalog shape is documented
- validation error shape is documented
- valid examples exist
- invalid golden cases exist
- one package target can validate those cases locally
- no package publish is required for local tests
