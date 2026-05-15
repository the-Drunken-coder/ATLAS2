# Sync Method Contract

## Purpose

Keep SDK local reads fresh without forcing every caller to perform direct API
requests.

Sync is one system made of server-filtered subscriptions, service events,
SDK-owned in-memory caches, periodic refresh, and reconnect recovery.

Broad current-state sync is a broad subscription preset. It is not a separate
replica-mode architecture.

## Method Families

### `atlas.sync.subscribe`

Intent: register one server-filtered subscription scope.

Minimum input:

- scope type
- scope selector
- optional cache behavior
- optional refresh interval override

Expected output:

- subscription handle or subscription ID
- initial refresh status, if the method performs an immediate refresh

Mode:

- sync setup

Subscription variants should be scope values passed to this method, not separate
top-level SDK methods. For example, subscribing to task records assigned to an
asset and subscribing to object metadata by object ID should both use
`atlas.sync.subscribe(scope, options)`.

### `atlas.sync.unsubscribe`

Intent: remove one active subscription scope.

Minimum input:

- subscription handle or subscription ID

Expected output:

- acknowledgement

Mode:

- sync control

### `atlas.sync.start`

Intent: start receiving service events for active subscription scopes.

Minimum input:

- optional connection options

Expected output:

- event connection state or handle

Mode:

- event stream setup

### `atlas.sync.close`

Intent: stop receiving service events and close sync resources.

Minimum input:

- optional reason

Expected output:

- acknowledgement

Mode:

- sync control

### `atlas.sync.refresh`

Intent: repair or initialize local cache state for one subscription, one
resource, or all active subscriptions.

Minimum input:

- refresh target: subscription, resource, or all active subscriptions

Expected output:

- refresh result summary

Mode:

- direct request used by sync internals
- exposed for explicit repair when useful

### `atlas.sync.get`

Intent: read current local cache state for a subscribed scope or resource.

Minimum input:

- resource type and identity, or subscription scope

Expected output:

- cached DTO, cached DTO list, or cache miss
- freshness metadata

Mode:

- local memory only
- should not pretend cached data is guaranteed fresh

### `atlas.sync.health`

Intent: report sync connection, subscription, cache, and refresh state.

Minimum input:

- none, or one subscription handle

Expected output:

- connection state
- active subscriptions
- last event time
- last refresh time
- drift or error state when known

Mode:

- local SDK state, optionally enriched by service status calls

## Initial Subscription Scopes

These are scope values for `atlas.sync.subscribe(...)`, not separate methods:

- tasks assigned to an asset
- task updates by task ID
- entity updates by entity ID
- track telemetry by track ID
- observation updates by observation ID
- object metadata updates by object ID
- object file metadata updates by object ID
- broad current-state scope for rich clients

Assets should generally use narrow scopes. They should not run the broad
current-state preset as the normal operating model.

## Event Payload Contract

Events should identify:

- event ID
- event type
- resource type
- resource ID
- mutation type
- server timestamp
- subscription scope that matched, if useful
- optional resource snapshot or structured diff

Create and update events should include enough data for the SDK to update local
memory without immediately refetching the changed resource.

The event producer may send a full resource snapshot or a structured diff. For
small resources, a full snapshot may be simpler and smaller. For larger JSON
resources, a diff may be more efficient.

Application code should read local state through SDK methods instead of handling
raw diffs directly.

Delete events can identify the deleted resource without carrying the old
resource snapshot.

Object file events should not include file bytes. They should update object or
file metadata and let callers fetch content separately when needed.

## Refresh Contract

Sync should support:

- startup refresh for selected subscription scopes
- server-filtered event updates while connected
- periodic scoped refresh, likely every 20-30 seconds by default
- full refresh after reconnect or suspected drift
- targeted refresh for one resource when needed

Refresh is the repair path when an event is missed, arrives out of order, or
cannot be applied cleanly.

## API Capabilities Required

- server-filtered subscription capability
- service event delivery for subscribed scopes
- scoped refresh capability
- targeted resource refresh capability
- broad current-state refresh capability for rich clients
- event payload shape that supports snapshots and diffs
- cache repair behavior after reconnect or suspected drift

## Non-Goals

Initial sync does not require:

- durable local storage
- durable server-side replay
- exactly-once delivery
- offline mutation queues
- arbitrary query languages
- per-application polling loops
- event-sourced Core storage
