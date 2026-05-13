# Change Events

## Status

Deferred.

Atlas Protocol should eventually define change event documents, but the first
protocol milestone does not need to settle event delivery.

## Current Direction

The event shape should be useful to Atlas Core, future APIs, and internal
workers, but delivery mechanisms are out of scope for now.

Delivery mechanisms may later include:

- Server-Sent Events
- streaming RPC
- an in-process fan-out hub
- Postgres notifications
- an outbox table

Those are implementation choices, not protocol requirements for the first
milestone.

## Likely Event Concept

The first protocol event will likely describe:

- resource family
- operation
- resource ID
- timestamp
- optional metadata

## Deferred Questions

- Should initial events be identifier-only or include post-state snapshots?
- How should object manifest/cache changes be represented?
- Should idempotent replays produce events?
- How much event metadata is required for clients to re-fetch state safely?

These questions are not blocking the first resource/command-catalog protocol
milestone.
