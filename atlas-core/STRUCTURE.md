# Atlas Core Structure Plan

This note captures the intended direction for organizing `atlas-core` as the
Vertical Slice 2 code grows. The current layout is workable, but `internal/`
is visually flat: runtime wiring, domain models, ports, validation, storage
adapters, and service behavior all sit at the same level. That makes the
system feel more tangled than its actual dependency graph.

The immediate goal is clarity, not a broad rewrite. Prefer staged mechanical
moves that preserve behavior and keep PRs reviewable.

## Current `atlas-core/` Shape

```text
atlas-core/
  cmd/atlas-core/          # binary entrypoint
  docker/                  # container support
  internal/                # all private application code
  Dockerfile
  docker-compose.yml
  go.mod
  go.sum
```

This top-level shape is mostly fine. `atlas-core` is the Go module root, and
Docker/runtime files live beside the module they run. The main issue is inside
`internal/`, not the module root.

If the module grows, the preferred top-level shape is:

```text
atlas-core/
  cmd/atlas-core/          # binary entrypoint only
  docker/                  # container support
  internal/                # private Go packages
  STRUCTURE.md             # architecture and layout notes
  Dockerfile
  docker-compose.yml
  go.mod
  go.sum
```

Avoid adding unrelated root-level directories unless they represent a real new
surface area, such as public API definitions or generated artifacts.

## Current `internal/` Shape

```text
internal/
  app/                     # runtime assembly and lifecycle
  blobvalidation/          # JSON blob normalization and validation
  config/                  # environment-backed config
  envutil/                 # small env helpers
  function/                # application behavior / use cases
  logging/                 # structured logger
  manifestvalidation/      # object manifest validation
  model/                   # domain structs and errors
  objectstorage/           # filesystem object storage adapter
  postgres/                # Postgres stores and schema setup
  store/                   # persistence/storage interfaces
  testsupport/             # test helpers
```

The package boundaries are not fundamentally wrong, but the directory listing
does not communicate the architecture. A reader has to infer which packages are
core domain code, which are adapters, which are runtime wiring, and which are
support utilities.

The biggest local problem is `internal/function/function.go`: it combines
entity functions, object lifecycle, object file operations, reconciliation,
task command semantics, observation functions, idempotency helpers, and shared
validation in one large file.

## First Reorganization Pass

Keep the package path stable and split files within `internal/function` first:

```text
internal/function/
  function.go              # Functions aggregate and constructors
  entity.go                # EntityFunctions
  object.go                # ObjectFunctions metadata lifecycle
  object_files.go          # object file read/write/list/delete
  object_reconcile.go      # object manifest repair and reconciliation
  task.go                  # TaskFunctions and command catalog semantics
  observation.go           # ObservationFunctions
  idempotency.go           # idempotency options/helpers
  validation.go            # model-level validation helpers
```

Then split the large tests the same way:

```text
internal/function/
  entity_test.go
  object_test.go
  object_files_test.go
  object_reconcile_test.go
  task_test.go
  observation_test.go
  validation_test.go
  function_integration_test.go
```

This pass should be mechanical: move code, preserve package names, avoid import
churn outside `internal/function`, and prove the move with `go test ./internal/function`.

## Target `internal/` Shape

After the low-risk split, consider grouping packages by role:

```text
internal/
  runtime/
    app/                   # process assembly, lifecycle, readiness
    config/                # env-backed config
    logging/               # logger
    envutil/               # env helpers

  core/
    model/                 # domain structs, typed errors, shared constants
    ports/                 # interfaces currently in store/

  service/
    entity.go
    object.go
    object_files.go
    object_reconcile.go
    task.go
    observation.go
    idempotency.go
    validation.go

  validation/
    blob/                  # current blobvalidation/
    manifest/              # current manifestvalidation/

  adapters/
    postgres/              # current postgres/
    objectstorage/         # current objectstorage/

  testsupport/
```

This layout makes the dependency direction explicit:

```text
cmd -> runtime/app -> service -> core/ports + core/model
service -> validation
adapters -> core/ports + core/model
runtime/app -> adapters
```

The important rule is that adapters should not own domain policy. Postgres and
filesystem storage should implement contracts; service code should own behavior
such as task semantics, idempotent create behavior, object/file metadata rules,
and reconciliation policy.

## Migration Guidance

Do this in small PRs:

1. Split `internal/function/function.go` without changing behavior.
2. Split `internal/function` tests to match the new files.
3. Rename `function` to `service` only if the split lands cleanly.
4. Move `store` to `core/ports` if the service package rename is accepted.
5. Move validation and adapters into grouped folders only after the earlier
   steps are stable.

Avoid doing the full tree move in the same PR as behavior changes. If a move is
mechanical, keep it mechanical and validate it with targeted tests plus
`go test ./...`.
