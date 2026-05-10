# ATLAS2

Command-and-control platform for managing distributed assets over constrained communication links.

## Local workflow

- `python3 atlas.py protocol-sync` builds Atlas Protocol and syncs validator artifacts into `atlas-core/protocol/`.
- `python3 atlas.py protocol-check` is the pre-commit/pre-push verification for stale or broken protocol artifacts and fails if `atlas-protocol/` and `atlas-core/protocol/` are out of sync.
- `python3 atlas.py start` rebuilds/syncs Atlas Protocol before starting the Docker stack.
- Atlas Protocol lives in `atlas-protocol/` and is the source of truth for caller-owned JSON shapes.
