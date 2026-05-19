# 0001 — HTTP API boundary: idempotency and row version

**Status:** Accepted
**Date:** 2026-05-07 (idempotency scope clarified 2026-05-18)

## Context

Persistence and the function layer already implement **idempotency keys** (scoped
dedup for creates) and **integer `version`** columns (optimistic concurrency on
updates). The public HTTP API is not wired in `atlas-core` yet; until it exists,
Atlas Core has no supported remote public product API.

**Scope of this ADR:** the **future public HTTP product edge** only. It does not
forbid idempotency on the internal platform API.

Today, optional `idempotency_key` on `ObjectRequest` and `TaskRequest` flows
through `atlas-functions` gRPC into the function layer (`WithIdempotencyKey`),
which claims keys in Postgres via datastorage idempotency primitives. That
internal path is documented here so it is not confused with the HTTP contract.

## Decision

- **Ownership:** Idempotency mechanics and version checks stay in the **store +
  function** layers. The HTTP edge does not become the source of truth for those
  rules.
- **Public HTTP idempotency:** **Not part of the public HTTP API.** Remote HTTP
  callers must not send idempotency keys, headers, or body fields. HTTP handlers
  do not pass `WithIdempotencyKey` from remote client input. Retries without
  server-side dedup may create duplicates unless the HTTP edge adds internal
  correlation later.
- **Internal functions gRPC:** Optional `idempotency_key` on create-object and
  create-task is **supported for co-located internal callers** and is **not** a
  public product contract. Future REST or bridge wrappers may add
  server-generated internal correlation for retry handling, but must not accept
  caller-provided idempotency keys from HTTP headers or request bodies.
- **datastorage:** Stores idempotency claims only; no product-facing key
  semantics.
- **Version:** Avoid exposing raw `version` integers in public JSON if we want a
  cleaner contract; prefer **ETag / If-Match** at the HTTP layer mapping to
  `model.*.Version` internally, or **explicit last-write-wins** at the edge if we
  accept overwriting without client-visible preconditions.

## Consequences

- New HTTP code should use **DTOs or mapping** at the boundary so `model` types
  with `json:"version"` are not serialized to clients by accident.
- Internal platform callers may rely on gRPC idempotency keys for safe create
  retries; document retry semantics when adding new co-located consumers.
- Removing `idempotency_key` from shared proto messages is optional cleanup and
  not required for HTTP alignment; only the HTTP edge must stay key-free for
  remote callers.

## Supersedes

—  

## Superseded by

—
