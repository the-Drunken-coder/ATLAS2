# Atlas Protocol runtime artifacts for Atlas Core

This directory is a synced runtime mirror used by Atlas Core. It does not contain the editable `atlas-protocol/` source package.

## What lives here

- `atlas-protocol-validator.mjs`: bundled validator CLI executed by Atlas Core.
- `schema-bundle.json`: synced schema bundle for runtime and tooling checks.
- `types.ts`: generated TypeScript types from Atlas Protocol schemas.
- `validators/index.mjs`: bundled standalone validator exports.

## Local workflow

Do not edit files in this directory by hand. Regenerate and sync them from the source package with:

```bash
python3 atlas.py protocol-sync
```

`python3 atlas.py start` also rebuilds and syncs these artifacts before the local stack starts.

The full editable Atlas Protocol package lives in `../atlas-protocol/`.
