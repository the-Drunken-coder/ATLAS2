# Rejected Or Deferred Ideas

## Multi-Language Parity

The old connection package set had Python and TypeScript clients plus parity
tests. That created maintenance cost.

Do not carry that forward. Start with one TypeScript SDK.

## Legacy Endpoint Names

The old helper contains endpoint names and workflow shortcuts from Atlas
Command.

Do not copy them directly. Use current Atlas Core resources and Atlas Protocol
shapes as the source of truth.

## Hidden Mutation Retries

The SDK should not hide failed writes behind automatic retries.

If Core returns an error, callers should see it. Workflow helpers can be
idempotent only when that behavior is explicit.

## Durable Local Sync Cache

SDK sync caches should be in-memory only.

Do not add browser storage, files, IndexedDB, or offline-first persistence in
this phase.
