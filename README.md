# ATLAS2

Command-and-control platform for managing distributed assets over constrained communication links.

## Atlas Core

Service boundaries, API entrypoints, and changefeed: [ADR 0002](docs/atlas-core/design-decisions/0002-service-boundaries-grpc-changefeed.md). Compose reachability and exposure: [ADR 0003](docs/atlas-core/design-decisions/0003-internal-api-exposure-posture.md).

- `python3 atlas.py start` runs gRPC code generation and starts the local compose stack (`postgres`, `atlas-datastorage`, `atlas-functions`).
