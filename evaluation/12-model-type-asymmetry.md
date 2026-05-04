# 12 — Type asymmetry across the model package

## Fix complexity

**Low.** A few hours. The compiler walks you through every call site; the tests confirm.

## Issue

Some enum-shaped fields in `model/types.go` are typed (`EntityType`, `OwnerType`, `TaskStatus`) and some aren't (`Object.Type` is plain `string`), so the compiler catches typos in some places but not others.

## In depth

`model/types.go`:

- `Entity.Type` is `EntityType` — typed, with `EntityTypeAsset`, `EntityTypeTrack`, `EntityTypeGeofeature` constants.
- `Object.OwnerType` is `OwnerType` — typed, with `OwnerTypeEntity`, `OwnerTypeObservation`, `OwnerTypeTask`, `OwnerTypeSystem`.
- `Task.Status` is `TaskStatus` — typed, with `TaskStatusPending`, `TaskStatusAcknowledged`, `TaskStatusCompleted`, `TaskStatusFailed`.
- `Object.Type` is `string` — **not typed**. Anything goes.

Object types referenced in the codebase: `"command_catalog"`, `"log"`, `"photo"`, plus whatever shows up in tests. There's no central list, no compile-time check, no validation that a given object type is one of the known set.

This matters for the `tasks.command_catalog_object_id` foreign-key relationship (see issue 14) — the column is supposed to point at an object whose `type = 'command_catalog'`, but neither the DB nor the Go layer knows the type name is `"command_catalog"` rather than `"commandCatalog"` or `"command-catalog"`. A typo in one place compiles cleanly and inserts a row that's silently inconsistent with the rest of the system.

## Recommended fix

1. Define `model.ObjectType` as a string newtype.
2. Add typed constants for every known object type (`ObjectTypeCommandCatalog`, `ObjectTypeLog`, `ObjectTypePhoto`, etc.) — even if the spec leaves the set open-ended, the *known* values should be named.
3. Change `Object.Type` to `ObjectType`.
4. Update `validateObjectModel` to check membership against the known set (or, if the set really is open-ended, at least apply a charset / length rule).
5. Add a `CHECK` constraint on `objects.type` if the set is closed (see issue 13 for the broader CHECK story).
