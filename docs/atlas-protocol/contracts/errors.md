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
  "code": "OUT_OF_RANGE",
  "message": "must be between -90 and 90"
}
```

Fields:

- `field`: dot-separated field path rooted at the submitted document or
  protocol context
- `code`: stable machine-readable error code
- `message`: concise human-readable explanation

The validator may return one error or an array of errors. The protocol-level
result shape is:

```json
{
  "ok": false,
  "errors": [
    {
      "field": "json.components.command.type",
      "code": "REQUIRED",
      "message": "is required"
    }
  ]
}
```

Valid results may include normalized output:

```json
{
  "ok": true,
  "normalized": "{\"components\":{\"parameters\":{},\"command\":{\"type\":\"hold_position\"}}}"
}
```

## Field Paths

Field paths should be deterministic.

Examples:

- `json`
- `json.type`
- `json.components`
- `json.components.telemetry.latitude`
- `json.components.supported_commands.commands`
- `json.latest_sighting.observed_at`
- `command_catalog.commands[0].id`
- `command_catalog.commands[0].parameters_schema.latitude.type`

Array indexes use bracket notation.

## Initial Error Codes

The initial protocol should support these codes:

- `INVALID_JSON`
- `INVALID_TYPE`
- `REQUIRED`
- `UNKNOWN_FIELD`
- `DUPLICATE_FIELD`
- `DUPLICATE_ID`
- `PROMOTED_FIELD`
- `RESERVED_FIELD`
- `OUT_OF_RANGE`
- `INVALID_VALUE`
- `TOO_LARGE`
- `TOO_DEEP`
- `TOO_MANY_FIELDS`
- `KEY_TOO_LONG`

Implementations may include additional internal details while debugging, but
published protocol-facing errors should preserve `field`, `code`, and
`message`.

## Conformance

Golden invalid cases should assert exact `field` and `code`.

Golden tests may assert exact `message` for stable protocol messages, but field
and code are the stronger compatibility contract.
