# ATLAS2 Codebase Evaluation

Each issue lives in its own file. Files are numbered roughly by severity: 01–08 are foundational/architectural, 09–17 are mid-level, 18 is a grab-bag of smaller items.

## Foundational

- [01 — Cross-layer atomicity faked with manual rollbacks](01-cross-layer-atomicity.md)
- [02 — Function layer is a near-empty pass-through and untested](02-function-layer-untested.md)
- [03 — Path-traversal hardening is incomplete](03-path-traversal-incomplete.md)
- [04 — Manifest writes are not atomic](04-non-atomic-manifest-writes.md)
- [05 — Error model mixes pointer-equality with `%w` wrapping](05-error-model-brittle.md)
- [06 — App is a god struct with no enforced mutation boundary](06-app-god-struct.md)
- [07 — Filesystem-as-source-of-truth is half-implemented](07-fs-source-of-truth-half-implemented.md)
- [08 — Concurrency story is empty](08-empty-concurrency-story.md)

## Mid-level

- [09 — Validation duplicated across layers](09-duplicated-validation.md)
- [10 — Inconsistent test-DB safety guards](10-inconsistent-test-db-guards.md)
- [11 — Mixed timestamp representations](11-mixed-timestamp-types.md)
- [12 — Type asymmetry in models](12-model-type-asymmetry.md)
- [13 — No DB-level CHECK constraints on enum-like columns](13-no-db-check-constraints.md)
- [14 — Task FK enforces existence, not type](14-task-fk-doesnt-enforce-type.md)
- [15 — `CreateObjectFolder` has dead code and weak idempotency](15-create-folder-dead-code.md)
- [16 — Logging is structured at the envelope but not the payload](16-unstructured-log-payloads.md)
- [17 — Docker / runtime nits](17-docker-runtime-nits.md)

## Smaller issues

- [18 — Grab-bag of smaller issues](18-smaller-issues.md)
