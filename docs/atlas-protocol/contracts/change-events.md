# Change Events

## Purpose

Change events describe successful Atlas resource mutations in a package-neutral
shape that Atlas Core, future APIs, internal workers, and clients can share.

Atlas Protocol owns the event document shape. Delivery mechanisms stay outside
the protocol and may include Server-Sent Events, streaming RPC, in-process
fan-out, Postgres notifications, or an outbox table.

## Event Shape

Every change event has:

```json
{
  "type": "change_event",
  "event_id": "evt-entity-asset-001-created-v1",
  "resource": "entity",
  "operation": "created",
  "resource_id": "asset-001",
  "resource_version": 1,
  "occurred_at": "2026-01-01T00:10:00Z",
  "snapshot": {
    "entity_id": "asset-001",
    "entity_type": "asset",
    "version": 1,
    "created_at": "2026-01-01T00:10:00Z",
    "updated_at": "2026-01-01T00:10:00Z",
    "json": {
      "components": {
        "supported_commands": {
          "commands": ["move_to_location"]
        }
      }
    }
  },
  "metadata": {
    "source": "atlas-core"
  }
}
```

Required fields:

- `type`: must be `change_event`
- `event_id`: non-empty event identifier
- `resource`: one of `entity`, `object`, `task`, or `observation`
- `operation`: one of `created`, `updated`, or `deleted`
- `resource_id`: identifier of the changed resource
- `resource_version`: resource version after the mutation; for deletes, the
  version that was deleted
- `occurred_at`: RFC 3339 timestamp
- `snapshot`: row-plus-json snapshot for `created` and `updated`, or `null` for
  `deleted`

Optional fields:

- `metadata`: object for producer metadata

## Snapshot Shape

Snapshots include promoted row fields plus the caller-owned protocol `json`.
The `json` member must validate against the corresponding Atlas Protocol
resource contract.

Entity snapshots:

- `entity_id`
- `entity_type`: `asset`, `track`, or `geofeature`
- `subtype` (optional)
- `alias` (optional)
- `version`
- `created_at`
- `updated_at`
- `json`

Object snapshots:

- `object_id`
- `object_type`: `log`, `photo`, or `document`
- `owner_type`: `entity`, `observation`, `task`, or `system`
- `owner_id`
- `version`
- `created_at`
- `updated_at`
- `json`

Task snapshots:

- `task_id`
- `status`: `pending`, `acknowledged`, `completed`, or `failed`
- `asset_id`
- `command_catalog_object_id`
- `version`
- `created_at`
- `updated_at`
- `json`

Observation snapshots:

- `observation_id`
- `source_asset_id`
- `version`
- `created_at`
- `updated_at`
- `json`

## Runtime Semantics

Atlas Protocol validates event shape only. Implementations decide when to emit
events, how to deliver them, whether delivery is durable, and how idempotent
replays behave. Failed mutations must not be represented as successful change
events.
