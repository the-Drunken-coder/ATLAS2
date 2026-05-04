# 13 — No DB-level CHECK constraints on enum-like columns

## Fix complexity

**Low.** Half an hour. The destructive-reset workflow means there's no migration story to manage.

## Issue

`entities.type`, `objects.owner_type`, and `tasks.status` are all `TEXT NOT NULL` with no `CHECK (... IN (...))` constraint, so the only thing keeping bad values out is Go-side validation — which any code path bypassing the function layer (or any manual `psql` insert) can skip.

## In depth

The schema in `postgres/schema.go` defines these columns as plain `TEXT`. The valid values live only in `model/types.go`:

- `EntityType`: `"asset" | "track" | "geofeature"`
- `OwnerType`: `"entity" | "observation" | "task" | "system"`
- `TaskStatus`: `"pending" | "acknowledged" | "completed" | "failed"`

The function layer validates these (`validateEntityModel:472`, `validateTaskModel:512`). The store layer doesn't. Postgres doesn't. Anyone running a manual `INSERT INTO entities ... VALUES ('asset_1', 'asst', ...)` (typo) gets a successful row that violates the model's invariants.

Combined with issue 06 (stores reachable from any code holding `*App`) and issue 12 (`Object.Type` not even typed in Go), this creates several routes for bad data:

- API handler (when one exists) reaches into `app.Stores` directly, bypassing function-layer validation.
- A buggy migration helper, dev script, or test setup `INSERT`s with a typo.
- A future bulk-import path forgets to validate.

The function layer can't be the only line of defense here. The DB is supposed to be one too.

## Recommended fix

Add `CHECK` constraints to the schema:

```sql
ALTER TABLE entities ADD CONSTRAINT entities_type_check
    CHECK (type IN ('asset', 'track', 'geofeature'));
ALTER TABLE objects ADD CONSTRAINT objects_owner_type_check
    CHECK (owner_type IN ('entity', 'observation', 'task', 'system'));
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check
    CHECK (status IN ('pending', 'acknowledged', 'completed', 'failed'));
```

Bake them into the `CREATE TABLE` statements in `schema.go` directly so they apply on every fresh init. Since the spec explicitly says no migrations during this phase (`SPEC.md:111`), and "stop = destructive reset," this is straightforward.

Optional: if the `objects.type` set ends up closed (see issue 12), add a CHECK there too.
