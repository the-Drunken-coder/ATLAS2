# Transport

## Direction

Use a fetch-based HTTP transport.

The SDK should work in modern Node and browser-like runtimes. It should use
`globalThis.fetch` by default and allow callers to inject a custom fetch
implementation for tests or nonstandard runtimes.

## Client Configuration

Initial configuration should include:

- `baseUrl`
- optional auth token or auth header provider
- optional `fetch`
- request timeout with a conservative default
- optional timeout override, including an explicit disable path for long-lived
  requests when the caller chooses it intentionally

Authentication should be easy to disable in local development because Core auth
is planned as a configurable system, not a hard requirement for early dev.

## Timeout Policy

The SDK should not leave requests unbounded by default.

Every request should run with a default timeout so callers do not hang forever on
network failures or unavailable Core instances. The client can expose
configuration to override that timeout globally or per request, and it can allow
an explicit opt-out for intentionally long-lived operations, but the default
contract should always be bounded.

## HTTP Wrapper Responsibilities

The internal HTTP wrapper should centralize:

- URL joining
- query parameter encoding
- JSON request encoding
- raw byte request handling for files
- multipart handling if selected for uploads
- response parsing
- empty response handling
- error conversion
- auth header attachment

Resource clients should call this wrapper instead of building `fetch` calls
directly.

## Raw Escape Hatch

Expose a limited raw request escape hatch for tools and debugging.

The raw escape hatch should be secondary. Normal application code should use
typed resource clients and sync helpers.

## No Hidden Mutation Retries

Do not add automatic retries for writes.

Retries can duplicate side effects or hide Core errors. If a write fails, the
SDK should surface the error. Higher-level helpers may do explicit read-first
flows only when that behavior is part of the helper contract.
