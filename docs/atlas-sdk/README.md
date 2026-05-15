# Atlas SDK Planning

This folder plans the TypeScript client package that will sit in front of Atlas
Core's HTTP API.

Working name: **Atlas SDK**.

The previous phrase "Atlas Connection Package" describes the job, but it is a
weak product/package name. "Atlas SDK" is shorter, easier to say, and matches
the role: the main developer-facing way to use Atlas Core.

## Purpose

The SDK should define the client-facing surface first. Atlas Core's HTTP API
should then be built as the bridge between that surface and the existing Core
function layer.

The SDK should make normal Atlas client code easy without hiding Core semantics
or inventing a second business-logic layer.

## Structure

- `infrastructure/`: package structure, transport, errors, testing, build, and
  background lessons.
- `features/`: one small file per SDK feature or feature family.

Use `infrastructure/` when changing how the SDK is built or wired.

Use `features/` when changing what the SDK does for callers.

## Core Decisions So Far

- Build only a TypeScript/npm package for now.
- Do not build Go, Rust, Python, or parity-tested multi-language clients.
- Use modern `fetch` with injectable fetch for tests and nonstandard runtimes.
- Keep Core authoritative for validation, runtime checks, storage, and protocol
  enforcement.
- Keep SDK validation lightweight and ergonomic.
- Design SDK methods before freezing the API route contract.
- Use SDK sync for server-filtered subscriptions, service events, local cache,
  and refresh.
- Treat broad current-state sync as a subscription preset, not a separate
  replica architecture.
- Do not use durable replay, exactly-once eventing, or per-read polling in the
  first sync design.
