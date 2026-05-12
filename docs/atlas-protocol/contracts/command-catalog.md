# Command Catalog

## Purpose

The command catalog defines the commands Atlas assets may execute and the
parameters each command accepts.

Atlas Protocol owns the catalog document shape. Atlas Core owns where the
catalog is stored, how it is loaded, and how task writes are checked against the
currently stored catalog.

## Source Material

This initial shape is derived from the earlier Atlas project:

- `ATLAS/Atlas_Command/command_catalog/command_catalog.json`
- `ATLAS/Atlas_Command/command_catalog/README.md`
- `ATLAS/Atlas_Command/scripts/seed_command_catalog.py`

That project stores the catalog as a JSON object with catalog metadata and a
`commands` array. The UI and consistency tests also expect string command IDs
and the plural field name `parameters_schema`.

## Catalog Document Shape

The initial Atlas Protocol command catalog document is:

```json
{
  "type": "command_catalog",
  "name": "Atlas Command Catalog",
  "description": "Preset catalog that describes the commands available to Atlas assets.",
  "commands": [
    {
      "id": "move_to_location",
      "name": "Move to Location",
      "description": "Command an asset to move to a specific geographic location.",
      "parameters_schema": {
        "latitude": {
          "type": "number",
          "description": "Target latitude in decimal degrees (WGS84).",
          "required": true
        },
        "longitude": {
          "type": "number",
          "description": "Target longitude in decimal degrees (WGS84).",
          "required": true
        }
      }
    }
  ]
}
```

Required catalog fields:

- `type`: must be `command_catalog`
- `name`: non-empty human-readable catalog name
- `description`: human-readable catalog description
- `commands`: array of command definitions

Required command fields:

- `id`: unique string command identifier
- `name`: non-empty human-readable command name
- `description`: human-readable command description
- `parameters_schema`: object describing accepted parameters

Command IDs should be URL-safe, lowercase, and use underscores for word
separation, such as `move_to_location` or `return_to_home`.

## Parameter Schema Shape

The initial parameter schema is the lightweight Atlas shape used by the earlier
Atlas command catalog. It is not full JSON Schema.

Each key under `parameters_schema` is a parameter name. Each parameter
definition may contain:

- `type`: one of `string`, `number`, `boolean`, `object`, or `array`
- `description`: human-readable parameter description
- `required`: boolean; `true` when the parameter must be supplied

Example:

```json
{
  "parameters_schema": {
    "geofeature_id": {
      "type": "string",
      "description": "The identifier of the geofeature entity to monitor.",
      "required": true
    }
  }
}
```

## Adding A Command

To add a command to the catalog:

1. Add one object to the catalog's `commands` array.
2. Choose a unique string `id`.
3. Provide `name` and `description`.
4. Define `parameters_schema`; use `{}` when the command accepts no parameters.
5. Add the command ID to each asset's `components.supported_commands.commands`
   list when that asset can execute the command.
6. Run protocol validation and golden tests.
7. Run any Atlas Core integration tests that check task validation against the
   stored catalog.

## Runtime Use

Task JSON references the command by ID:

```json
{
  "components": {
    "command": {
      "type": "move_to_location"
    },
    "parameters": {
      "latitude": 40.7141,
      "longitude": -74.0029
    }
  }
}
```

Atlas Core should then check:

- the task document is protocol-valid
- the target asset exists
- the asset supports `move_to_location`
- the stored command catalog exists
- the catalog has a command with `id = "move_to_location"`
- task parameters match that command's `parameters_schema`

## Array Versus Keyed Map

The catalog uses an array as the canonical protocol shape because that is what
the earlier Atlas catalog and UI consumed.

Consumers may derive an in-memory lookup map keyed by command `id` for faster
validation. That derived map is an implementation detail, not the wire/document
shape.

## Open Questions

- Should the lightweight `parameters_schema` later be replaced by or translated
  into full JSON Schema?
- Should command definitions include optional metadata such as category,
  display order, safety level, or target asset classes?
- Should protocol validation require every command ID to match a stricter regex?
