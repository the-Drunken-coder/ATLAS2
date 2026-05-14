# Validation Errors

## Purpose

Atlas Protocol validation errors should be stable enough for humans, clients,
agents, tests, and SDKs to act on.

The error shape should stay small and language-neutral.

## Shape

Every protocol validation error has:

```json
{
  "field": "json.components.telemetry.latitude",
  "code": "invalid_value",
  "message": "latitude is out of range"
}
```

Fields:

- `field`: dot-separated field path rooted at the submitted document or
  protocol context (the reference TypeScript validator prefixes document paths
  with `json.`)
- `code`: stable machine-readable error code (lowercase `snake_case` in the
  current reference implementation)
- `message`: concise human-readable explanation

The reference validator returns validation output as a **JSON array** of
`{ field, code, message }` objects (zero or more issues). The local `validate`
CLI prints that array with pretty JSON when validation fails, and prints `[]`
when the document is valid. There is no `{ ok, errors }` envelope and no
normalized success payload from the validator today.

Example failure output (array of issues):

```json
[
  {
    "field": "json.components.command.type",
    "code": "required",
    "message": "command.type is required"
  }
]
```

The machine schema for each issue is
[`validation-error.schema.json`](../../../atlas-protocol/source/schemas/validation-error.schema.json).

## Field Paths

Field paths should be deterministic.

Examples:

- `json`
- `json.type`
- `json.components`
- `json.components.telemetry.latitude`
- `json.components.supported_commands.commands`
- `json.latest_sighting.observed_at`
- `json.commands[0].id`
- `json.commands[0].parameters_schema.latitude.type`
- `resource` (when the resource kind itself is invalid for programmatic entry)

Array indexes use bracket notation.

## Error Codes (reference implementation)

The current TypeScript reference validator emits at least these `code` values:

- `invalid_json` — input is not valid JSON
- `invalid_type` — value is not the required JSON type (for example root must
  be an object)
- `required` — a required field is missing
- `required_pair` — paired fields must appear together (for example track
  latitude and longitude)
- `invalid_value` — value fails a semantic or range rule, or unknown resource
  kind
- `unknown_field` — property not allowed (including schema `additionalProperties`
  and forbidden aliases such as `parameter_schema` on command catalog entries)
- `promoted_field` — a field that belongs in promoted columns appears at the top
  level of entity-like JSON
- `reserved_field` — reserved object keys (for example manifest cache fields on
  objects)
- `duplicate_command_id` — duplicate `id` values in a command catalog `commands`
  array
- `limit_exceeded` — document or extension section exceeds size, depth,
  field-count, or key-length limits

Implementations may include additional internal details while debugging, but
published protocol-facing errors should preserve `field`, `code`, and
`message`.

Atlas Core may wrap protocol validation failures in a later integration phase,
but that wrapper must not discard, rename, or reinterpret the protocol issue
payload. The Protocol-level compatibility contract remains the
`{ field, code, message }` issue object.

## Conformance

Golden invalid cases should assert exact `field` and `code`.

Golden tests may assert exact `message` for stable protocol messages, but field
and code are the stronger compatibility contract.
