# 06 — App is a god struct with no enforced mutation boundary

## Fix complexity

**Low — but rises with delay.** Today it's about a day of plumbing in `app.go` and any internal callers. Once API handlers exist and reach into `app.Stores`, every one of them becomes a migration. Best done before the API layer lands.

## Issue

`app.App` exposes `Stores`, `Funcs`, `Pool`, and `ObjStore` as public fields, so the spec's "API handlers must call functions, not stores" rule is enforced by hope rather than by the type system — and the cost of fixing it rises every time a new caller is added.

## In depth

`app.go:18-25`:

```go
type App struct {
    Config   *config.Config
    Logger   *logging.Logger
    Pool     interface{ Close() }
    Stores   Stores
    Funcs    function.Functions
    ObjStore *objectstorage.Store
}
```

Every store and the underlying `*objectstorage.Store` is reachable from any code that holds an `*App`. When the API layer arrives in the next slice, nothing prevents a handler from doing `app.Stores.Object.CreateObject(...)` directly — bypassing every validation, rollback, log, and (eventually) audit hook the function layer is supposed to add. The spec says (`SPEC.md:386`): "Stores should not be called directly by public API handlers later. Normal system behavior should not bypass the function layer." That rule lives only in the spec.

A separate but related smell: `Pool interface{ Close() }`. The pool is anonymized to an interface that only exposes `Close()`, but only *after* the four stores have already captured the typed `*pgxpool.Pool`. The interface narrows nothing; it's a bit of theater that just makes `app.go` harder to read.

The cost dynamics matter: today it's one struct edit to fix. After two API handlers reach into `app.Stores` directly, it's a refactor across multiple files. After ten, nobody wants to touch it.

## Recommended fix

1. Make `Stores` private — either lowercase the field, or move the wiring into the `function` package and let `app` only hold `function.Functions`.
2. Drop the `Pool interface{ Close() }` indirection. Either keep the typed `*pgxpool.Pool` (and accept that its `Close()` is what `Shutdown` needs), or move shutdown ownership to a small `Closer` slice the app appends to.
3. Make `ObjStore` private; access it through `Funcs.Object` only.
4. Audit any planned API-layer wiring docs to make the function-only access explicit.
