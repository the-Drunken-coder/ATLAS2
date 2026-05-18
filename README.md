# ATLAS2

Command-and-control platform for managing distributed assets over constrained communication links.

## Atlas Core service layout

- `atlas-core/services/datastorage/` owns PostgreSQL, schema setup, object storage, and storage-integrity workflows.
- `atlas-core/services/functions/` owns protocol validation, orchestration, idempotent mutations, and the gRPC changefeed.
- `atlas-core/services/shared/` holds shared helpers plus generated protobuf/gRPC types.
- `python3 atlas.py start` runs gRPC code generation and starts the local compose stack (`postgres`, `atlas-datastorage`, `atlas-functions`).

`atlas-functions` is the only supported public API. External clients must never
call `atlas-datastorage` directly; datastorage is a private implementation
detail for the functions service and direct calls bypass business validation,
idempotency orchestration, and changefeed publication guarantees.
