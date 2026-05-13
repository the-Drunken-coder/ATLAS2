# Atlas Protocol Examples

These files are examples of valid caller-owned Atlas Protocol JSON documents.

**Conformance fixtures:** the TypeScript verifier copies these JSON files into
`atlas-protocol/examples/` on each `npm run build` (`prebuild`). That directory
is what `valid-examples.json` references; keep docs and copied examples aligned.

Each file exposes a `minimum` payload (smallest JSON that satisfies the
relevant create / full-update / upsert constraints) and one or more `full*`
payloads (maximal discovery-oriented variants such as `full`, `full_success`,
or `full_error`) as defined in
[`../contracts/resources.md`](../contracts/resources.md).

These examples are not database rows. Resource examples represent only the
value stored in the resource's `json` column; `custom-sections.json` is a
standalone reusable `custom_*` shape.

Promoted fields such as `entity_id`, `object_id`, `task_id`, `observation_id`,
`type`, `status`, `owner_type`, `owner_id`, `asset_id`, `source_asset_id`,
`command_catalog_object_id`, `created_at`, `updated_at`, and `version` must not
appear at the top level of these JSON blobs.

Required vs optional fields are documented in
[`../contracts/resources.md`](../contracts/resources.md).

JSON files in this folder must remain valid JSON and must not contain comments.

## File Purposes

- `assets.json`: `entity.json` for an asset. `minimum` is only required
  `supported_commands` (empty `commands` is valid but blocks task targeting until
  populated). The `full` example only includes optional `observed_at` on telemetry
  and heartbeat where delayed measurement or relay time matters most.
- `tracks.json`: `entity.json` for a track. `minimum` is required `telemetry`
  with both `latitude` and `longitude`. An optional
  `telemetry.uncertainty_radius_m` carries a display-friendly horizontal
  uncertainty radius (typically supplied by the data fusion system).
- `geofeatures.json`: `entity.json` for a geofeature. `minimum` is only required
  standard GeoJSON `geometry`.
- `tasks.json`: `task.json`. `minimum` is required `components.command.type` and
  `components.parameters` (empty object).
- `observations.json`: `observation.json`. `minimum` is only required `state`.
- `objects-log.json`: `object.json` for `log`. `minimum` is `{}`.
- `objects-photo.json`: `object.json` for `photo`. `minimum` is `{}`.
- `objects-document.json`: `object.json` for `document` (generic structured
  payload such as JSON, markdown, or XML; payload lives in object files). The
  command catalog is stored as a `document` with `id = command_catalog`.
  `minimum` is `{}`.
- `command-catalog.json`: command catalog document payload. The command catalog
  is stored by Atlas Core as the `document` object with `object_id =
  command_catalog`, but this example is only the protocol JSON payload. The
  `full` payload mirrors the legacy Atlas preset catalog when that local tree is
  available.
- `change-events.json`: change event documents with row-plus-json snapshots for
  create/update-style events and `snapshot: null` for deletes.
- `custom-sections.json`: standalone `custom_*` object shape (not a full resource
  envelope); `minimum` uses an empty `custom_vendor` object.

## Geofeature geometry shape variants

`geofeatures.json` shows one valid geometry (`Polygon`) in `full` and another
(`Point`) in `minimum`. Atlas Protocol validates standard GeoJSON geometry
shape and basic topology for `components.geometry`.

Standard GeoJSON shapes:

```json
{ "type": "Point", "coordinates": [-74.01, 40.71] }
```

```json
{
  "type": "LineString",
  "coordinates": [
    [-74.01, 40.71],
    [-74.00, 40.72],
    [-73.99, 40.73]
  ]
}
```

```json
{
  "type": "Polygon",
  "coordinates": [
    [
      [-74.01, 40.71],
      [-74.00, 40.71],
      [-74.00, 40.72],
      [-74.01, 40.72],
      [-74.01, 40.71]
    ]
  ]
}
```

Non-standard shape (not valid Atlas Protocol GeoJSON):

```json
{ "type": "Circle", "coordinates": [-74.01, 40.71, 250] }
```

`Circle` is not part of the GeoJSON spec. Keep circle radius or display hints in
`custom_*` extension sections unless a future protocol version defines a
standard extension.
