# 05 — Error model mixes pointer-equality sentinels with `%w` wrapping

## Fix complexity

**Low** but spread across many files. Half a day for the change. Adding a lint rule prevents the pattern from coming back.

## Issue

The codebase compares errors with `==` against package-level pointer sentinels (`err == model.ErrNotFound`) — a single innocent `fmt.Errorf("...: %w", err)` anywhere in the call chain silently breaks every one of those checks.

## In depth

Three coexisting error shapes:

- **Sentinel pointer errors:** `model.ErrNotFound`, `model.ErrConflict`, etc. are `*CoreError` package-level singletons (`model/errors.go:32-41`). Compared via `==`:
  - `function.go:228`: `if err == model.ErrNotFound`
  - `function.go:192`: `if existingErr != nil && existingErr != model.ErrNotFound`
- **`*FieldError`:** returned from validation.
- **Wrapped errors:** `fmt.Errorf("get object: %w", err)` from stores.

`*CoreError` has no `Unwrap` method and no `Is` method. So `errors.Is(err, model.ErrNotFound)` works *only* when `err` is the literal sentinel — wrapping breaks it. And the pointer compare (`err == model.ErrNotFound`) breaks for the exact same reason.

Today the stores happen to return `model.ErrNotFound` directly, unwrapped, so the `==` checks happen to work. But the moment anyone adds a wrap — a normal Go idiom and the kind of change a reviewer would not flag — every `==` comparison silently returns false. Symptom: `GetObject` on a missing ID starts returning a generic 500 instead of a clean not-found, or `GetObjectManifest`'s empty-manifest fallback (`function.go:228-230`) stops triggering and callers crash.

The bug is invisible at the call sites because nothing looks wrong locally.

## Recommended fix

1. Add an `Is(target error) bool` method to `*CoreError` that returns true when both errors have the same `Code`.
2. Replace every `err == model.Err...` in the codebase with `errors.Is(err, model.Err...)`. About 10–15 call sites.
3. Replace string-concat error messages in `function.go` (`function.go:127, 171, 211, 264`) with `%w` wrapping so the chain is preserved.
4. Add `errorlint` (or equivalent) to the project's lint config to prevent regressions.

Optionally: convert pointer sentinels to value sentinels (`var ErrNotFound = errors.New(...)`), since pointer-typed singletons are an unusual choice in Go and don't buy anything here.
