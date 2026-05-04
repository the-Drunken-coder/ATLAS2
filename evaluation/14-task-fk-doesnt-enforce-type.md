# 14 — Task FK enforces existence, not type

## Fix complexity

**Low.** Option A is half a day — add the lookup, plumb the error, write a test that asserts a non-command-catalog object_id is rejected. Option B is a deeper schema change and not worth it unless other reasons pile up.

## Issue

`tasks.command_catalog_object_id` is a foreign key into `objects(object_id)`, but neither the schema nor the function layer requires that the referenced object actually has `type = 'command_catalog'` — so a task can legally point at any object in the system.

## In depth

`postgres/schema.go:37`:

```sql
command_catalog_object_id TEXT NOT NULL REFERENCES objects(object_id),
```

The FK ensures the row *exists*. Nothing checks that `objects.type` equals `'command_catalog'` — not in the DB (no trigger or constraint), not in the function layer (`validateTaskModel` at `function.go:503-523` doesn't fetch the referenced object), not anywhere.

Concrete failure: a task can be created with `command_catalog_object_id = "some_log_object_id"`, and the system happily stores it. Any code that later assumes "the object behind a task's `command_catalog_object_id` is a command catalog" — which is what the field name and the spec both imply — gets a runtime surprise.

This is the same class of bug as issue 13 (no DB-level enum check) but for a relational invariant rather than an enum membership.

## Recommended fix

Two reasonable options:

**Option A — type-check at the function layer.** In `CreateTask` / `UpdateTask` / `UpsertTask`, fetch the referenced object and verify `obj.Type == "command_catalog"` before delegating to the store. Adds a round-trip per write but keeps the schema simple.

**Option B — model command catalogs as their own table.** `tasks.command_catalog_id` references `command_catalogs(id)` directly. The DB enforces both existence and type. The downside is breaking the "objects" abstraction the codebase has chosen — every kind of object is a row in `objects`, not a separate table.

Option A is the smaller change and matches the existing design. Either way, also strengthen this with a DB CHECK if the validation moves into a trigger.

A weaker but cheaper improvement: at least add a comment in the schema explaining the unenforced invariant, so the next person to read it doesn't assume the FK is enough.
