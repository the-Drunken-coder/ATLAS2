# Objects Method Contract

## Purpose

Manage Atlas objects as data artifacts with metadata, JSON, file metadata, and
optional file content.

The SDK should make the common one-object, one-primary-content case easy while
still allowing multi-file objects.

## Method Families

### `atlas.objects.create`

Intent: create one object record and its initial JSON metadata.

Minimum input:

- object type
- owner or parent fields required by Core
- object JSON
- optional manifest or file metadata if supported by the first API contract

Expected output:

- object info DTO

Mode:

- direct request mutation

### `atlas.objects.getInfo`

Intent: retrieve everything about an object except file bytes.

Minimum input:

- object ID

Expected output:

- object identity
- object type
- ownership
- object JSON
- created and updated timestamps
- manifest or equivalent file metadata

Mode:

- direct request by default
- may read from local cache when a matching object metadata subscription is
  active

### `atlas.objects.list`

Intent: browse or load a bounded set of object info records.

Minimum input:

- optional filters that Core explicitly supports
- optional bounds or pagination once the API contract defines them

Expected output:

- object info DTO list
- optional page or cursor metadata once defined

Mode:

- direct request by default
- broad current-state sync may serve rich clients through local cache once a
  matching subscription is active

### `atlas.objects.updateInfo`

Intent: update object metadata, object JSON, or manifest-level information.

Minimum input:

- object ID
- replacement or patch shape, once the API contract chooses update semantics

Expected output:

- updated object info DTO

Mode:

- direct request mutation

### `atlas.objects.delete`

Intent: delete one object record and the Core-owned object file state associated
with it.

Minimum input:

- object ID

Expected output:

- deletion acknowledgement or deleted object identity

Mode:

- direct request mutation

### `atlas.objects.getContent`

Intent: retrieve the primary object content or a selected file's content.

Minimum input:

- object ID
- optional file selector for multi-file objects
- optional desired return kind, such as JSON, text, bytes, or Blob

Expected output:

- parsed JSON, text, bytes, Blob, or equivalent runtime value
- content metadata needed by the caller

Mode:

- direct request
- object file bytes are not served from service events

This should remain one content-read method with options. Do not expose separate
top-level SDK methods for each content kind or file-selection variant unless
real usage proves that split is needed.

### `atlas.objects.putContent`

Intent: store or replace the primary object content or a selected file's
content.

Minimum input:

- object ID
- content
- optional file selector or file metadata
- content type, when known

Expected output:

- updated object info DTO or file metadata DTO

Mode:

- direct request mutation

This should remain one content-write method with options. Content type, primary
content, and selected-file writes should not become separate top-level SDK
methods by default.

## API Capabilities Required

- object info create, get, list, update, and delete capabilities
- object content read and write capabilities
- support for the common primary-content path
- support for selecting a file when an object has multiple files
- public object DTOs and file metadata DTOs
- Atlas Protocol validation error mapping for object JSON writes
- object-storage safety by going through Core functions
- subscription/event capability for object metadata and file metadata updates

## Current Core Notes

Object function-layer behavior exists. Command catalog objects should remain
ordinary object writes at the SDK/API boundary while Core continues validating
their JSON as Atlas Protocol `commandCatalog`.

Efficient time-sensitive object queries are deferred until Core persistence and
indexing needs are clearer.
