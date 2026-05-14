# Objects

## Purpose

Manage Atlas objects as user-facing data artifacts.

Objects combine:

- object metadata
- object JSON
- manifest/file metadata
- optional file content

The SDK should avoid exposing every low-level storage operation as a primary
method. Most callers want either object information or object content.

## Method Families

- create object
- get object info
- list objects
- update object info
- delete object
- get object content
- put object content

## Object Info

`get object info` should return everything about an object except file bytes in
one call.

That response should include:

- object identity
- object type
- ownership
- object JSON
- created/updated timestamps
- file manifest or equivalent file metadata

This avoids forcing callers to fetch object metadata and then make a separate
manifest call just to understand what the object contains.

## Object Content

`get object content` should retrieve the object's primary content.

Most objects are expected to have one meaningful file. For those objects, the SDK
should make content access straightforward:

- JSON objects can return parsed JSON.
- Text objects can return text.
- Binary objects such as images can return bytes, `Blob`, or an equivalent
  runtime-appropriate value.

The SDK can still expose lower-level file selection when an object has multiple
files, but that should be secondary to the common primary-content path.

## Multi-File Objects

Some objects may contain multiple files.

For those cases, the object info response should expose enough file metadata for
the caller to choose a file. Content methods can accept an optional file name or
file selector when the primary file is ambiguous.

Avoid making normal callers manually list files before every content read.

## Time-Sensitive Object Data

Some future object types may contain time-sensitive data that callers want to
query by time window or range.

Keep this on the table, but do not make it part of the first SDK object surface.
Efficient time-range queries may require backend storage/indexing changes, so
they should be designed with Core persistence rather than added as SDK-only
helpers.

## Notes

The SDK should hide low-level upload and download mechanics from normal callers.

Object file bytes should not be included in service event payloads.
