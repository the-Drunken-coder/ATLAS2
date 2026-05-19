---
name: Atlas service restructuring
overview: "Restructure ATLAS2 from a single atlas-core binary into two in-scope deployables—data storage (merged DB + volume + stores) and functions (validation + orchestration + gRPC to storage)—with protobuf/gRPC contracts, lockstep codegen via atlas.py, and a single network entry to functions (one listen addr; unary + subscription RPCs on one channel). HTTP API and browser SSE are explicitly a future plan (qualities documented elsewhere, not built here). Mutation events are implemented inside the functions tier but separated cleanly in code/proto from unary command paths; consumers in this plan are tests or ad-hoc gRPC clients, not an API service."
todos:
  - id: boundary-contract-design
    content: Pin datastorage/function RPC capabilities, idempotency ownership, error mapping, event shape, and reconcile visibility before codegen
    status: completed
  - id: contracts-codegen
    content: Add proto layout + buf/protoc codegen after boundary decisions; wire atlas.py start + CI to run codegen before compose build
    status: completed
  - id: datastorage-svc
    content: Create services/datastorage with its own cmd/internal packages; move postgres/objectstorage/store code; expose capability gRPC; keep schema-in-code init + storage-owned object workflows
    status: completed
  - id: functions-svc
    content: Create services/functions with its own cmd/internal packages; port function+protocolvalidation; gRPC client to datastorage; integration tests on compose network
    status: completed
  - id: changefeed
    content: Implement Publisher seam on mutation success after event-shape decision; expose subscription as gRPC server-stream on the same functions server/port; keep changefeed packages/proto surface separate from unary handlers; verify with integration tests or grpcurl (no HTTP API in this plan)
    status: completed
  - id: compose-atlaspy
    content: Split docker-compose/Dockerfiles/healthchecks for datastorage + functions (+ postgres); update atlas.py + AGENTS.md + integration workflow
    status: completed
  - id: adrs-docs
    content: Update design-decisions and vertical-slice docs for two-service reality; record deferred HTTP/SSE API plan, single-entry gRPC model, and VS3 transport pivot
    status: completed
  - id: fusion-deferred
    content: "Stub fusion phase: folder + contracts only when fusion scope is defined"
    status: pending
  - id: http-api-deferred
    content: "Future plan (out of scope here): services/api, HTTP JSON + SSE, one gRPC channel to functions for unary + stream"
    status: pending
isProject: false
---

> **Historical plan.** Restructuring is complete. Authoritative decisions live in
> [design-decisions/](../design-decisions/) ADRs 0001–0005. Do not treat this
> file as the source of truth for boundaries or exposure—read the ADRs.
> **Location:** Version-controlled copy of the restructuring plan (Cursor plan id `277b9383`).

# Atlas multi-service restructuring

## Summary

Split Atlas Core into two deployables under `atlas-core/services/`:

- **`atlas-datastorage`** — Postgres, schema-in-code, object volume, storage-integrity workflows
- **`atlas-functions`** — protocol validation, orchestration, idempotent mutations, internal platform gRPC, changefeed stream

One gRPC listen address on functions hosts unary RPCs and `SubscribeMutations` on the same channel. [`atlas.py`](../../../atlas.py) runs codegen before compose.

```mermaid
flowchart TB
  subgraph callers [Callers_in_this_plan]
    Tests[Integration_tests_and_tools]
  end
  subgraph logic [Functions_service_single_port]
    Unary[Unary_mutations_queries]
    Stream[ServerStream_subscription]
    Val[Protocol_validation]
  end
  subgraph persist [Datastorage_service]
    PG[pgx_Stores_and_volume]
  end
  subgraph infra [Compose_infra]
    PostgresDB[(Postgres_container)]
  end
  Tests -->|"one_gRPC_channel"| Unary
  Tests -->|"same_channel"| Stream
  Unary --> Val
  Stream --> Val
  Unary -->|gRPC| PG
  PG --> PostgresDB
```

## Authoritative documentation

| Topic | ADR |
|-------|-----|
| Service boundaries, gRPC entrypoints, changefeed, reconcile | [0002](../design-decisions/0002-service-boundaries-grpc-changefeed.md) |
| Compose reachability and exposure | [0003](../design-decisions/0003-internal-api-exposure-posture.md) |
| HTTP idempotency at the product edge | [0001](../design-decisions/0001-api-boundary-idempotency-versioning.md) |
| Single-tenant deployment | [0004](../design-decisions/0004-single-tenant-deployment-model.md) |
| Reset-first schema-in-code | [0005](../design-decisions/0005-reset-first-schema-in-code.md) |

## Out of scope (deferred)

- **HTTP API + browser SSE** — future `services/api/`; see [vertical-slice-3/SPEC.md](../vertical-slice-3/SPEC.md)
- **Data fusion** — future `services/fusion/` when scoped

## Repo layout (implemented)

- `atlas-core/services/datastorage/` — persistence binary and internal packages
- `atlas-core/services/functions/` — functions binary, changefeed, protocol validation
- `atlas-core/services/shared/` — generated gRPC types, shared model/store helpers
- `atlas-core/proto/` — internal RPC contracts; generated Go in `services/shared/gen/`
