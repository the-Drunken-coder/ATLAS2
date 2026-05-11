# atlas-protocol

Local Atlas Protocol implementation for schema-driven contract validation.

## Layout

- `source/schemas/`: JSON Schema source-of-truth contracts
- `source/manifests/`: validation manifests for valid and invalid golden cases
- `source/goldens/invalid/`: invalid golden payloads
- `packages/typescript/`: first package target consuming schema source
- `scripts/`: local verification entrypoints

## Local verification

```bash
python3 atlas.py protocol-check
```

That command compiles the TypeScript package and validates all current protocol
examples and golden failures without publishing anything.
