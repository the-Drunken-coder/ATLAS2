# Sync

> **Superseded for sync behavior** by [../sync-contract.md](../sync-contract.md).
> This file keeps exploratory notes only (scopes, diffs, server-filtered
> subscriptions). Do not treat it as the implementation contract.

## Purpose

Sync is the SDK feature that keeps local reads fresh without making every caller
perform direct API requests.

It combines:

- server-filtered subscriptions
- service events
- local in-memory caches
- periodic refresh
- reconnect recovery

These are one system, not separate feature families.

## Core Model

- Request mode: direct calls to Core API.
- Subscription: the server-side filter describing what data the client wants.
- Event: the delivery mechanism for changes matching a subscription.
- Local cache: the SDK-owned in-memory state for subscribed data.
- Broad subscription preset: subscribe to all normal current-state data.

What we previously called replica mode is just the broad subscription preset. It
should not be a separate architecture.

## Scope Choices

Clients choose how much data they subscribe to:

- narrow scope: one task view, one entity, one observation, one object, or one
  small related set
- broad scope: all normal current-state resources needed by a command
  interface, dashboard, map, or debugging tool

Assets should generally use narrow scopes. They should receive only data
relevant to their current role, subscribed task views, sensors, or requested
context.

Command interfaces and dashboards may use the broad scope when they genuinely
need the full current-state view.

## Method Families

- subscribe to a scope
- unsubscribe from a scope
- start event sync for active scopes
- close event sync
- read from local cache when a matching scope is active
- refresh subscribed scopes
- report sync health

## Initial Subscription Scopes

- tasks assigned to an asset
- task updates by task ID
- entity updates by entity ID
- track telemetry by track ID
- observation updates by observation ID
- object metadata updates by object ID
- object file metadata updates by object ID
- broad current-state scope for rich clients

Area, type, or query-shaped subscriptions may be useful later.

## Events

Events should identify:

- event ID
- event type
- resource type
- resource ID
- mutation type
- server timestamp
- optional resource payload or resource diff

Create and update events should include enough information for the SDK to update
local memory without immediately refetching the changed resource.

That payload can be:

- a full resource snapshot
- a structured diff against the previous resource JSON

The event producer should choose the cheaper useful representation. For small
resources, sending the full resource may be simpler and smaller than a diff. For
larger JSON resources, sending only changed sections may be more efficient.

The SDK should treat both forms as internal synchronization data. Application
code should read the resulting local resource state through the SDK instead of
handling raw diffs directly.

Delete events can identify the deleted resource without carrying the deleted
resource snapshot.

Object file events should not include file bytes. If an image or other large
object file changes, the event should update metadata and let callers fetch
content separately when they need it.

## Refresh

Events do not remove the need for refresh.

The SDK should support:

- startup refresh for the selected subscription set
- server-filtered event updates while connected
- periodic scoped refresh, likely every 20-30 seconds by default
- full refresh after reconnect or suspected drift
- targeted refresh for one resource when needed

Refresh remains the repair path when a diff is missed, arrives out of order, or
cannot be applied cleanly.

## Task Subscription Pattern

The SDK should not own task execution, scheduling, or queue management.

Those behaviors belong to the system using the SDK. The SDK's job is to provide
the subscription and local-cache mechanism that lets that system avoid polling
Core for relevant task records.

Expected flow:

1. A system subscribes to task records relevant to itself.
2. Core sends only matching events.
3. The SDK updates the local cache for that scope.
4. The system's task-read function returns from that cache instead of making a
   fresh API call every time.
5. The SDK periodically performs a scoped refresh for that subscription to
   repair missed events or drift.

## Non-Goals

Initial sync behavior does not require:

- durable replay
- exactly-once delivery
- offline queues
- local durable storage
- arbitrary query languages
- per-application polling loops
- event-sourced storage model
