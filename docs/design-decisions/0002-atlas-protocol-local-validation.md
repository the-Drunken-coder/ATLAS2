# 0002 — Atlas Protocol as the local source of truth for caller-owned JSON

**Status:** Accepted  
**Date:** 2026-05-10

## Context

Atlas Core originally kept handwritten Go JSON-shape validators under `internal/validation/blob`. The Atlas Protocol extraction plan moves reusable caller-owned JSON contracts into a standalone package without changing Atlas Core's ownership of storage, object manifests, or cross-resource semantics.

We also need a local workflow that works before any public package is published and that keeps Atlas Core's runtime validator aligned with the schemas and examples used by protocol tooling.

## Decision

- Keep Atlas Protocol in the repository as a standalone local package at `atlas-protocol/`.
- Make `atlas-protocol/schemas/*.schema.json` the source of truth for caller-owned JSON document shapes.
- Use Atlas Protocol's TypeScript/Ajv validator from Atlas Core through `atlas-core/internal/protocolvalidation`.
- Keep Atlas Core semantic validation in Go for live-system rules such as resource existence, command catalog lookups, parameter validation against the stored catalog, and manifest behavior.
- Use `atlas.py` to build Atlas Protocol and sync the runtime artifacts into `atlas-core/protocol/` before Atlas Core starts or tests run.

## Consequences

- Node.js and pnpm become required local tooling for Atlas Protocol development and for Atlas Core workflows that validate caller-owned JSON.
- Atlas Core no longer keeps a second handwritten production validator for entity/object/task/observation JSON shapes, which reduces drift risk.
- Docker and CI must prepare Atlas Protocol artifacts before running Atlas Core or its Go tests.
- Atlas Core currently invokes the protocol validator as a short-lived Node subprocess on write paths; if profiling shows this becoming a bottleneck, revisit the integration with a persistent validator process rather than reintroducing a second validation layer.
