# Boundaries

## SDK Owns

- developer ergonomics
- typed request and response shapes
- simple request validation
- error conversion
- event subscription helpers
- optional local in-memory sync caches

## SDK Does Not Own

- Atlas Protocol schema definitions
- Core business rules
- Core runtime validation
- persistence behavior
- command execution
- API route implementation

## Type Source

Start with manually maintained TypeScript types based on the current docs and
Atlas Protocol contracts.

Do not add code generation until the API and protocol contracts are stable
enough to justify it.
