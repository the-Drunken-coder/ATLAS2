# atlas-protocol

Local Atlas Protocol implementation for schema-driven contract validation (TypeScript + JSON Schema).

## Layout

- `source/schemas/`: JSON Schema contracts
- `source/manifests/`: valid/invalid case manifests
- `source/goldens/invalid/`: invalid golden payloads
- `examples/`: JSON examples (copied from `docs/atlas-protocol/examples` on each `npm run build`)
- `packages/typescript/`: `AtlasProtocolValidator` and exports
- `scripts/`: `verify`, `validate`, `export-schema-bundle`, `sync-examples-from-docs`, `copy-protocol-dist`
- `dist/protocol/`: standalone bundle after build (`schemas`, `manifests`, `goldens`, `examples`); set `ATLAS_PROTOCOL_ROOT` to this directory to verify without reading repo `docs/`

## Version

The npm `version` field is the **Atlas Protocol version** for this tree (see [Versioning Policy](../docs/atlas-protocol/conformance.md#versioning-policy) in the protocol docs).

## Local verification

```bash
python3 atlas.py protocol-check
```

Runs `npm run verify` (build, sync examples, copy `dist/protocol`, run goldens and valid fixtures).

```bash
npm run verify:standalone
```

Runs the same checks with `ATLAS_PROTOCOL_ROOT=dist/protocol` only (no `docs/` reads at verify time).

## Validate an arbitrary file

From repo root (paths resolve against repo root if not found from cwd):

```bash
python3 atlas.py protocol-validate --resource commandCatalog --file path/to/catalog.json
```

Wrapped example files can be validated by selecting an example key:

```bash
python3 atlas.py protocol-validate --resource commandCatalog --file atlas-protocol/examples/command-catalog.json --example full
```

Or:

```bash
cd atlas-protocol && npm run validate -- --resource task --file examples/tasks.json --example minimum
```

Without `--example`, the JSON file must be the **resource root object**.

## Local linking

Use a `file:` dependency or `npm link` against this package. Pass the absolute `atlas-protocol` directory as `atlasProtocolRoot` when constructing `AtlasProtocolValidator(repoRoot, atlasProtocolRoot)`.

## Schema bundle

```bash
npm run bundle
```

Writes `generated/schema-bundle.json` (schemas keyed by basename + `protocolVersion`).
