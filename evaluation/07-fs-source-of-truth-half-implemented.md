# 07 — Filesystem-as-source-of-truth is half-implemented

## Fix complexity

**Medium.** Each piece is simple, but the design (when does reconciliation run, what's the orphan policy, how is the version stamp computed) takes thought. Probably 3–5 days end-to-end if done together with issue 01, less if you piggyback on whatever scaffolding that fix introduces.

## Issue

The spec says the filesystem manifest is canonical and the DB is a cache, but the codebase ships only the read path of that contract — there's no reconciliation function, no drift detection, no startup scrub, and `UpdateObjectManifest` uses a different clock than the rest of the writes.

## In depth

What's implemented:

- `function.GetObjectManifest` (`function.go:226`) reads from the filesystem first. ✅

What's missing or inconsistent:

1. **No reconciliation function.** Spec at `SPEC.md:347-348`: "implementations should provide a repair/reconciliation function that rebuilds the database `objects.json` column from the filesystem manifest." Nowhere in the code.
2. **No drift detection.** No function compares the FS manifest to the DB cache and reports differences.
3. **No startup scrub.** Nothing on boot looks for orphan FS folders (no DB row) or orphan DB rows (no FS folder). After any of the failure modes from issue 01, the orphans accumulate forever.
4. **Two clocks for `updated_at`.** Almost every write path sets `updated_at` from Go (`time.Now().UTC()`). But `ObjectStore.UpdateObjectManifest` (`object_store.go:204`) uses SQL `NOW()`. So a manifest write refreshes the row's `updated_at` to the DB clock, while the in-memory `obj.UpdatedAt` is unchanged. Subtle and inconsistent.
5. **Stale cache reads have no signal.** The whole reason for the DB cache is to query manifest data without filesystem access (`SPEC.md:332-333`). But `ListObjects` returns DB rows whose `json["manifest"]` may be stale relative to FS — and there's nothing on the row to indicate it might be stale, no version stamp, no checksum.

The net effect: callers who use the cache (the entire point of having it) get potentially stale data with no way to tell, and the system has no mechanism to converge to consistency.

## Recommended fix

A coherent fix touches several places, but each piece is small:

1. **Build the reconcile function.** Walks every object folder on disk, reads `manifest.json`, upserts the cache key in `objects.json["manifest"]`. Run on startup. Optionally run on a timer.
2. **Add an orphan scrub.** Two passes: FS folders without a matching DB row → log/repair (decide policy: delete folder vs upsert metadata vs surface for human attention); DB rows without an FS folder → fatal per spec (`SPEC.md:353`).
3. **Unify the clock.** Replace `NOW()` in `UpdateObjectManifest` with the Go-supplied timestamp, and add the same field to `ObjectManifest` writes from the function layer.
4. **Add a manifest version stamp** (e.g. a hash of file list) on both sides. Cheap drift detection: if FS hash ≠ cached hash, the DB row is known stale.

Coordinate this with the issue-01 fix; both want a reconcile worker.
