# 02 — The function layer is a near-empty pass-through, and its coordination logic is essentially untested

## Fix complexity

**Medium.** The mechanical refactor (interface-based deps + fakes + rollback tests) is 2–3 days. The harder part is the architectural call about whether the function layer should be retained as-is, thinned, or invested in. The code is easy; the decision is the bottleneck.

## Issue

The function layer is positioned by the spec as the "mutation boundary," but today it's mostly copy-paste validate-and-delegate code, and the only parts that actually coordinate Postgres + filesystem (Create/Delete/Upsert object) have **zero unit tests covering the rollback paths**.

## In depth

`function.go` is 569 lines, and the entity / task / observation paths are pure copy-paste:

- validate
- set timestamps to UTC if zero
- default `JSON` to `[]byte("{}")`
- log a single message
- delegate to the store

There is no event emission, no audit hook, no real coordination — except in `CreateObject`, `DeleteObject`, and `UpsertObject`, which are the only functions that touch two systems.

The unit tests do not exercise that coordination. Look at `function_test.go:21`:

```go
f := EntityFunctions{}
err := f.CreateEntity(nil, entity)
```

The function struct is zero-initialized — the embedded `*postgres.EntityStore` is nil, so is the logger, so is the context. The test only works because validation runs first and returns before reaching the store. The moment validation passes, those tests would nil-dereference.

Net result:

- Rollback paths in `CreateObject` / `DeleteObject` / `UpsertObject` have no unit-test coverage.
- The integration tests (`function_integration_test.go`) only cover `GetObjectManifest` and `UpdateObjectManifest`.
- When the next coordination step lands (event emission, multi-store writes), there is no test scaffolding for it. Every regression in failure paths will surface only in production.

There's also a structural blocker to fixing this: the function structs hold *concrete* pointers (`*postgres.EntityStore`, `*objectstorage.Store`) rather than the `store.X` interfaces that already exist in `store/store.go`. So fakes can't be substituted without first refactoring the dependencies.

## Recommended fix

The mechanical part:

1. Change function structs to depend on `store.EntityStore`, `store.ObjectStore`, etc. (interfaces already defined).
2. Write fake implementations of the four stores plus `objectstorage.Store` (or split that interface out too — `store.ObjectStorageStore` exists already, but `objectstorage.Store` is the concrete consumed type).
3. Write rollback tests that inject failures: "FS create fails", "FS create succeeds but rollback fails", "DB delete succeeds, FS delete fails, restore fails", etc.

The architectural call:

Decide whether the function layer is actually pulling its weight right now. If event emission, audit logging, and multi-step workflows are coming soon — yes, build the test infrastructure now. If not, thin the layer down to just the coordinating object operations and let the simple stores be called more directly. Don't carry a 569-line file of pass-through code on the chance it'll be useful.
