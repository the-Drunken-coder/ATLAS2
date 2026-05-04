# 08 — Concurrency story is empty

## Fix complexity

**Low to medium, depending on scope.** The tx helper plus configurable pool is half a day. The per-object mutex is another half-day. Optimistic concurrency is a schema change plus updates to every mutating store method — a couple of days. Total: 3–4 days for a complete story, but the pieces can ship independently.

## Issue

The codebase has no per-object locking, no transaction pattern, and no idempotency tokens — so two concurrent operations on the same object can interleave in ways that produce torn writes, duplicated work on retry, or DB↔FS divergence that the rollback paths can't unwind.

## In depth

Specific gaps:

1. **No per-object serialization.** Two concurrent `WriteManifestFile` calls for the same object: last-writer-wins at best, torn write at worst (see issue 04). `WriteObjectFile` and `AppendObjectFile` have the same shape — concurrent appends to the same log file from two callers will interleave bytes.
2. **No transaction pattern.** `postgres.NewPool` is set up but no code calls `BeginTx`. There are no multi-row writes today, but the moment one is needed (e.g. "write task and update its parent object"), there's no shared `tx`-aware helper to reach for.
3. **`MaxConns = 8` hardcoded.** `db.go:19`. Independent of workload, env, or expected concurrency. Will need to be configurable.
4. **No idempotency tokens.** A retried Create after a network blip can produce a successful-then-rolled-back state, then a duplicate-key error on retry, then a confused operator. Standard fix is a client-supplied `idempotency_key` checked on the server side.
5. **No optimistic concurrency.** No `version` or `updated_at` `WHERE` clauses on Update. Two concurrent `UpdateEntity` calls clobber each other silently — last write wins regardless of who read what.

This is "no concurrency story yet" rather than "broken concurrency." Acceptable for an internal foundation slice. But the patterns to support it (tx helper, per-key locking, idempotency keys, version columns) aren't in place, and they're easier to add now than after the API layer is built on top.

## Recommended fix

Phased:

1. **Now (cheap, valuable):** Add a `Begin` / `WithTx` helper to the postgres package so future multi-row work has somewhere to live. Make `MaxConns` configurable via env.
2. **Before the API layer:** Add a per-object mutex (in-process `sync.Map` of `*sync.Mutex` keyed by object ID) for FS operations on the same object. Cheap insurance against torn writes from concurrent callers.
3. **When operations get more interesting:** Add a `version` column on `entities`, `objects`, `tasks`, `observations` and switch updates to `WHERE entity_id = $1 AND version = $current`. Idempotency keys can ride on top of the same machinery.

The per-process mutex strategy assumes a single Atlas Core instance — fine for the local-first design — but worth a comment so it's not load-bearing for a future multi-instance deployment.
