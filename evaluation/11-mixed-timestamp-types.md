# 11 — Mixed timestamp representations across the codebase

## Fix complexity

**Low.** A few hours. The compiler will catch most call sites, the tests will catch the rest. Most of the changed code is stores and filter helpers.

## Issue

The same logical value — "a timestamp" — is represented as `time.Time`, `*string`, plain `string`, and SQL `NOW()` in different parts of the code, with no validation at the boundaries between them.

## In depth

Four representations in active use:

1. **`time.Time`** — `model.Entity.UpdatedAt`, `model.Object.UpdatedAt`, etc. Correct primary representation.
2. **`*string`** — `store.EntityFilterState.UpdatedAfter` and the equivalent fields on `Object`, `Task`, `Observation` filter states (`store/store.go:23, 55, 87, 103, 138`). These are bound directly into Postgres parameters as strings (`updated_at > $N` with a `*string` arg) and rely on Postgres's implicit cast from string to `TIMESTAMPTZ`. There's no Go-side validation that the string is even RFC3339; a typo propagates to the DB and produces a SQLSTATE error rather than a clean validation message.
3. **plain `string`** — `model.ObjectFileInfo.UpdatedAt` (the per-file timestamp inside the manifest). JSON-encoded as a string with no enforced format.
4. **SQL `NOW()`** — `object_store.go:204` in `UpdateObjectManifest`. Sets `updated_at` from the database clock instead of the application clock that every other write uses.

Real consequences:

- A poorly-formatted `UpdatedAfter` filter from a future API call returns a database error to the user instead of a 400.
- An `UpdateObjectManifest` call leaves the in-memory `obj.UpdatedAt` out of sync with the DB row — different value than every other update path produces.
- A clock skew between the app container and the DB container is silently absorbed in some paths and not others.
- Any cross-record comparison ("which was updated more recently?") gives different answers depending on whether you read the in-memory model or the row.

## Recommended fix

1. Use `time.Time` everywhere internally.
2. Change every `UpdatedAfter *string` filter to `*time.Time`. Validation moves to the API boundary (when it exists) — parse RFC3339, return a clean validation error on bad input.
3. Change `ObjectFileInfo.UpdatedAt` to `time.Time`, accept the JSON marshalling cost (it's still RFC3339 on the wire).
4. Replace `NOW()` in `UpdateObjectManifest` with the Go-supplied `time.Now().UTC()` from the function layer — or, better, pass the timestamp through the function call so the function layer remains the single source of clock truth.
