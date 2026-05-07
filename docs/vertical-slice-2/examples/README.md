# JSONB Validation Examples

These files are examples of valid caller-owned JSON blobs for Vertical Slice 2.

Each file exposes two payloads under top-level keys `full` (maximal shape for
discovery) and `minimum` (smallest JSON that satisfies the relevant create / full-update
constraints in [`../component-contracts.md`](../component-contracts.md)).

These examples are not database rows. They represent only the value stored in
the resource's `json` column.

Promoted fields such as `entity_id`, `object_id`, `task_id`, `observation_id`,
`type`, `status`, `owner_type`, `owner_id`, `asset_id`, `source_asset_id`,
`command_catalog_object_id`, `created_at`, `updated_at`, and `version` must not
appear at the top level of these JSON blobs.

Required vs optional fields are documented in
[`../component-contracts.md`](../component-contracts.md).

JSON files in this folder must remain valid JSON and must not contain comments.

## File Purposes

- `assets.json`: `entity.json` for an asset. `minimum` is only required
  `supported_commands` (empty `commands` is valid but blocks task targeting until
  populated). The `full` example only includes optional `observed_at` on telemetry
  and heartbeat where delayed measurement or relay time matters most.
- `tracks.json`: `entity.json` for a track. `minimum` is `{}` (no required
  entity components for track).
- `geofeatures.json`: `entity.json` for a geofeature. `minimum` is only required
  `geometry` (`type` + `coordinates`).
- `tasks.json`: `task.json`. `minimum` is required `components.command.type` and
  `components.parameters` (empty object).
- `observations.json`: `observation.json`. `minimum` is only required `state`.
- `objects-command-catalog.json`: `object.json` for `command_catalog`. `minimum`
  is `{}` (all catalog fields optional in JSONB).
- `objects-log.json`: `object.json` for `log`. `minimum` is `{}`.
- `objects-photo.json`: `object.json` for `photo`. `minimum` is `{}`.
- `custom-sections.json`: standalone `custom_*` object shape (not a full resource
  envelope); `minimum` uses an empty `custom_vendor` object.
