# 0001 — HTTP API boundary: idempotency and row version

**Status:** Proposed  
**Date:** 2026-05-07

## Context

Persistence and the function layer already implement **idempotency keys** (scoped dedup for creates) and **integer `version`** columns (optimistic concurrency on updates). An external HTTP API is not wired in `atlas-core` yet; when it exists we want a clear rule for what callers see.

## Decision

- **Ownership:** Idempotency mechanics and version checks stay in the **store + function** layers. The HTTP edge does not become the source of truth for those rules.
- **Idempotency:** **Not part of the public API.** Callers must not send idempotency keys, headers, or body fields. Any dedup or replay safety stays **internal** (server-generated correlation if we ever need it, or we simply accept duplicate creates on retry as an acceptable tradeoff). Handlers do not pass `WithIdempotencyKey` from client input.
- **Version:** Avoid exposing raw `version` integers in public JSON if we want a cleaner contract; prefer **ETag / If-Match** at the HTTP layer mapping to `model.*.Version` internally, or **explicit last-write-wins** at the edge if we accept overwriting without client-visible preconditions.

## Consequences

- New HTTP code should use **DTOs or mapping** at the boundary so `model` types with `json:"version"` are not serialized to clients by accident.
- Without a client-visible idempotency contract, **retries may create duplicates** unless we add purely internal dedup later; that is an explicit product tradeoff.

## Supersedes

—  

## Superseded by

—
