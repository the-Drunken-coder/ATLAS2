# Errors

## Direction

SDK calls should throw typed SDK errors.

Use one primary error class first. Add subclasses only when repeated caller code
proves they are useful.

## Error Fields

The SDK error should preserve:

- HTTP status
- Core error code
- Core error ID if provided
- message
- path if provided
- timestamp if provided
- details
- Atlas Protocol validation issues
- original cause where useful

## Protocol Validation Issues

Atlas Protocol validation issues must remain structured.

The SDK should not flatten protocol issues into one string. Preserve:

- `field`
- `code`
- `message`

## Client-Side Validation

SDK validation should stay lightweight:

- required arguments
- obvious ID emptiness checks
- obvious enum checks
- impossible option combinations

Core remains authoritative for Atlas Protocol validation and runtime checks.

