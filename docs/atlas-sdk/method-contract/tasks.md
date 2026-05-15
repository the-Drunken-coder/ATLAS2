# Tasks Method Contract

## Purpose

Manage task records through the SDK.

Tasks are data records. The SDK surfaces task data and subscribed task views; it
does not execute, schedule, queue, dispatch, or complete tasks on behalf of a
system.

## Method Families

### `atlas.tasks.create`

Intent: create one task record.

Minimum input:

- target asset or entity reference required by Core
- task JSON

Expected output:

- created task DTO

Mode:

- direct request mutation

### `atlas.tasks.get`

Intent: retrieve one task record by identity.

Minimum input:

- task ID

Expected output:

- task DTO

Mode:

- direct request by default
- may read from local cache when a matching task subscription is active and the
  caller chooses cache-backed reads

### `atlas.tasks.list`

Intent: browse or load bounded task records.

Minimum input:

- optional filters that Core explicitly supports
- optional bounds or pagination once the API contract defines them

Expected output:

- task DTO list
- optional page or cursor metadata once defined

Mode:

- direct request by default
- narrow task subscriptions should serve systems that repeatedly read their own
  task view

### `atlas.tasks.update`

Intent: update one task record.

Minimum input:

- task ID
- replacement or patch shape, once the API contract chooses update semantics

Expected output:

- updated task DTO

Mode:

- direct request mutation

### `atlas.tasks.delete`

Intent: delete one task record.

Minimum input:

- task ID

Expected output:

- deletion acknowledgement or deleted task identity

Mode:

- direct request mutation

## API Capabilities Required

- task create, get, list, update, and delete capabilities
- public task DTOs instead of direct `internal/model` serialization
- Atlas Protocol validation error mapping for task writes
- Core runtime validation error mapping for target assets, supported commands,
  command catalogs, and command parameters
- server-filtered task subscription scopes

## Current Core Notes

Task function-layer behavior exists. The API should call that layer and preserve
the current runtime checks.

Do not add SDK task execution helpers or queue management helpers.
