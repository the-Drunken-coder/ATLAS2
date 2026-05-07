# JSONB Validation Examples

These files are examples of valid caller-owned JSON blobs for Vertical Slice 2.

They are intentionally maximal examples: optional fields are included so agents
and developers can see the full supported shape.

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

- `asset.full.json`: maximal valid `entity.json` for an asset.
- `track.full.json`: maximal valid `entity.json` for a track.
- `geofeature.full.json`: maximal valid `entity.json` for a geofeature.
- `task.full.json`: maximal valid `task.json` with command, parameters,
  progress, result, and error sections.
- `observation.full.json`: maximal valid `observation.json` with state,
  latest sighting, sighting history pointer, extra, and custom data.
- `object-command-catalog.full.json`: maximal valid `object.json` for a
  `command_catalog` object.
- `object-log.full.json`: maximal valid `object.json` for a `log` object.
- `object-photo.full.json`: maximal valid `object.json` for a `photo` object.
- `custom-section.full.json`: standalone `custom_*` shape showing allowed
  extension structure.
