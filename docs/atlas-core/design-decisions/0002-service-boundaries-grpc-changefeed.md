# 0002 Service boundaries, gRPC entrypoint, and changefeed ownership

## Status

Accepted.

## Context

Atlas Core no longer fits comfortably as one binary that owns PostgreSQL,
filesystem object storage, protocol validation, and caller-facing mutation
behavior. The restructuring plan requires replaceable service seams, one caller
entrypoint to function behavior, and a changefeed that reflects successful
mutations without introducing an HTTP API yet.

## Decision

- Split Atlas Core into two Go services under `atlas-core/services/`:
  - `datastorage` owns PostgreSQL access, schema setup in code, filesystem
    object storage, and storage-integrity workflows such as object reconcile.
  - `functions` owns validation, orchestration, idempotent mutation semantics,
    and the caller-facing gRPC surface.
- Use protobuf/gRPC for the `functions -> datastorage` seam and for the single
  caller-facing entrypoint into `functions`.
- Keep unary mutation/query RPCs and the mutation subscription stream on the
  same `atlas-functions` gRPC server and listen address.
- Publish changefeed events from the functions layer after successful outer
  mutations. The first implementation uses an in-process gRPC stream hub and a
  shared event envelope that carries resource, operation, version, and optional
  snapshot data.
- Keep HTTP/JSON and browser SSE as a later API-layer concern. They are not part
  of this restructuring deliverable.

## Consequences

- Datastorage remains the only normal writer of SQL rows and object files.
- Functions can be replaced independently as long as they continue to implement
  the protobuf contracts and mutation publication behavior.
- Local startup and CI now require gRPC code generation before build/compose.
- Cross-service integration shifts toward compose/gRPC verification instead of
  direct same-process package coupling.
