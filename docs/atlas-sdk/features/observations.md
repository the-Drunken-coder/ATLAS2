# Observations

## Purpose

Manage observation records.

Observations may become high-volume data. The SDK should expose method families
that let callers ask for useful slices of observation data without forcing them
to manually scan everything.

## Method Families

- create
- get
  - by ID
  - by time or time window
  - by source asset
  - by related track or target
  - by source asset and related track or target
- list
- update
- delete

## Get And List

`get` retrieves observations by intent.

It should support:

- a known observation ID
- observations at or around a specific timestamp
- observations within a start/end time range
- observations produced by one source asset
- recent observations produced by one source asset
- observations related to a specific track or target
- observations from one source asset about one track or target

`list` is the general browsing method and should support bounded results.

High-volume consumers should prefer specific `get` options instead of listing
all observations and filtering locally.

## Time Basis

Observation JSON has `latest_sighting.observed_at`, while Core rows also have
server-owned timestamps. The API/SDK contract should be explicit about which
time basis each `get` option uses.

Core already models `source_asset_id` as a promoted field, so source-asset get
options are natural first-class reads.

Track or target get options should exist once the observation-to-track/target
relationship is explicit in the data model.

## Smart Observation Reads

Observation reads should be smart at the SDK/API boundary.

That means:

- callers use intent-shaped methods instead of manually assembling broad scans
- the SDK does not hide expensive unbounded reads behind convenient names
- API/Core implementation can add indexes or promoted fields as needed
- high-volume query methods should be designed before callers depend on
  inefficient local filtering

## Notes

Do not add specialized observation reporting helpers until the public SDK/API
observation and object-content paths are stable.
