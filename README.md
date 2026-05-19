# ATLAS2

Command-and-control platform for managing distributed assets over constrained communication links.

## Atlas Core service layout

- `atlas-core/services/datastorage/` owns PostgreSQL, schema setup, object storage, and storage-integrity workflows.
- `atlas-core/services/functions/` owns protocol validation, orchestration, idempotent mutations, and the gRPC changefeed.
- `atlas-core/services/shared/` holds shared helpers plus generated protobuf/gRPC types.
- `python3 atlas.py start` runs gRPC code generation and starts the local compose stack (`postgres`, `atlas-datastorage`, `atlas-functions`).

## API layers

Until the HTTP API exists, Atlas Core has **no supported remote public product API.**

| Layer | Service | Role |
|-------|---------|------|
| Product edge (future) | HTTP REST API | The public HTTP API is the product edge. Auth, TLS, and rate limits belong here. |
| Internal platform | `atlas-functions` gRPC | `atlas-functions` is the internal platform API. Co-located Atlas components (future REST gateway, analytics, other on-host services) call it; it is not internet-facing. |
| Private persistence | `atlas-datastorage` gRPC | External clients must never call `atlas-datastorage` directly. Direct calls bypass business validation, idempotency orchestration, and changefeed publication guarantees. |

### Reachability (default compose)

Default `docker-compose.yml` is **Docker-internal only** for functions: co-located containers on `atlas-internal` dial `atlas-functions:8080`. Host-native tools (grpcurl, IDE) are **not** reachable on localhost unless you use the integration/debug compose override (`python3 atlas.py start-debug`) or run services natively on the host.
