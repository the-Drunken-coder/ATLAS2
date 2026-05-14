# Testing

## Test Goals

The SDK should prove:

- request construction
- query parameter handling
- JSON body encoding
- raw file request behavior
- response parsing
- error conversion
- typed protocol issue preservation
- service event parsing
- public method behavior

## Mocked Transport First

Most SDK tests should use an injected fake fetch implementation.

That keeps tests fast and avoids requiring Atlas Core to be running for normal
package tests.

## Integration Tests

Add a smaller set of integration tests once the Core API exists.

Those tests should prove the SDK can talk to a real Atlas Core instance for the
main resource flows, but they should not replace the faster mocked transport
tests.

## No Cross-Language Parity Suite

The old Atlas package set needed parity tests because it had Python and
TypeScript clients.

This system should not carry that burden forward. With a TypeScript-only SDK,
test effort should focus on SDK behavior and SDK/Core contract compatibility.

## Contract Drift Checks

Once API routes exist, add tests or fixtures that make drift obvious between:

- SDK request shapes
- Core API route expectations
- Atlas Protocol resource shapes
- Core API error envelopes

