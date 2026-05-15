# Carry Forward

Useful ideas from older Atlas packages and docs.

## One Primary Client

Carry forward one primary `AtlasClient` that owns base URL handling, headers,
request construction, response parsing, and resource namespaces.

## Fetch Injection

Allow callers to inject `fetch`. This keeps tests simple and supports
nonstandard runtimes.

## Central HTTP Wrapper

Request construction, JSON encoding, response parsing, and error conversion
should live in one internal HTTP module.

## Optional Auth Header

The SDK should be able to include auth details when Core auth is enabled, while
local development can run with auth disabled.

## Resource Helpers

Keep grouped resource methods for entities, objects, object files, tasks,
observations, service status, and sync.

## Typed Errors

Preserve the Core API error envelope and Atlas Protocol validation issues in SDK
errors.

## Request Mode And Sync

Request mode maps calls to Core API requests.

Sync keeps in-memory current-state caches from scoped refreshes and
server-filtered events. A broad current-state cache is just a broad subscription
preset.

## Server-Filtered Subscriptions

Sync should not depend on receiving all Core events and filtering locally. The
SDK should declare interest and Core or the bridge should send only matching
event traffic.

## Event Recovery By Refresh

Events are live-only. Recover missed state with scoped or full refreshes, not
durable event replay.
