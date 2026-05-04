# 18 — Smaller issues

## Fix complexity

**Trivial.** A couple of hours, mostly mechanical. The value is review hygiene, not architecture.

## Issue

A grab-bag of small code-quality and hygiene issues: a `mustMarshal` panic, copy-pasted `envOrDefault`, a too-thin `.gitignore`, formatting inconsistencies, and a few smaller smells. Individually trivial; collectively a signal of where review hygiene is loose.

## In depth

### `mustMarshal` panics in service code

`objectstorage/store.go:342-348`:

```go
func mustMarshal(v interface{}) []byte {
    data, err := json.Marshal(v)
    if err != nil {
        panic(fmt.Sprintf("marshal manifest: %v", err))
    }
    return data
}
```

Used only on a literal `model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}`, where `json.Marshal` is provably infallible. So the panic is unreachable in practice. But a `must*` helper that panics in a long-running service is a smell — it's easy to repurpose later for an input that *can* fail to marshal, and then the panic ships.

### `envOrDefault` is duplicated

`config/config.go:69-74` and `postgres/testutil_test.go:67-72` define the same two-line helper. Move it to a shared internal package or just export it from `config`.

### `.gitignore` is two lines

`.gitignore`:

```
.env
*.log
```

Doesn't ignore Go test artifacts (`*.test`), coverage files (`coverage.out`, `coverage.html`), local binaries (`atlas-core` if anyone runs `go build` at the package), IDE files (`.idea/`, `.vscode/`). Easy to leak local junk into a commit.

### `model/types.go:11` has odd alignment

```go
EntityTypeAsset       EntityType = "asset"
EntityTypeTrack       EntityType = "track"
EntityTypeGeofeature  EntityType = "geofeature"  // ← extra space
```

`gofmt` would not touch this (alignment of constant values isn't enforced), but it's ugly.

### Filter `*string` for timestamps fails inside Postgres

Already covered in detail in [issue 11](11-mixed-timestamp-types.md), but worth restating in the smaller-issues bucket: a typo'd timestamp string in a `WithObjectUpdatedAfter("not a date")` call produces a SQLSTATE error rather than a clean validation error.

### `app.Pool interface{ Close() }`

Already covered in [issue 06](06-app-god-struct.md). The interface narrows nothing; remove it.

### Pointer-equality error compares

Already covered in [issue 05](05-error-model-brittle.md): `function.go:228` does `err == model.ErrNotFound`. Breaks the moment any layer adds a `%w` wrap.

### `Object.Type` is plain `string`

Already covered in [issue 12](12-model-type-asymmetry.md).

### `ObjectFileInfo.UpdatedAt` is plain `string`

Already covered in [issue 11](11-mixed-timestamp-types.md).

## Recommended fix

Most of these items are one-liners or near-it. Roll them into a single "code hygiene" PR:

1. Replace `mustMarshal` with explicit error handling at the one call site, or remove it entirely if the call site is guaranteed-safe and the marshal is small enough to inline.
2. Move `envOrDefault` to `internal/config` (or a new `internal/envutil`) and import it from tests.
3. Expand `.gitignore` with the standard Go set: `*.test`, `coverage.out`, `coverage.html`, `/atlas-core`, `bin/`, `.idea/`, `.vscode/`.
4. Run `gofmt -s` (already idempotent on most of the codebase) and clean the alignment in `model/types.go`.
5. Rest is covered in the linked issues.
