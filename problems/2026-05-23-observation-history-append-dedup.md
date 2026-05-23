1. **Time & Date:** 2026-05-23T00:00:00Z
2. **Name:** History append-if-absent not safe across functions replicas
3. **Issue:** `appendHistoryEventIfAbsent` uses an in-process mutex per `historyObjectID`; concurrent same `event_id` in one process is deduped. Multiple functions instances against shared storage can both pass `historyContainsEventID` and append duplicate `history.ndjson` lines. Sidecar/index staleness is mitigated (history.ndjson fallback after `a186054`); gap tests for append-ok + sidecar-fail retry dedup and ingest-level concurrency remain.
4. **Severity:** S5 (Trivial/Documentation) — closed by design: owner will never run multi-replica atlas-functions (see Owner decisions); in-process mutex + `history.ndjson` authority is sufficient under ADR 0004.
5. **Location:** `atlas-core/services/functions/internal/function/observation_history_dedup.go`, `observation_history_reconcile.go`, `observation_history_events.go`; datastorage append path (no cross-process dedup)
6. **Expected:** For single-tenant single functions replica (ADR 0004 default): in-process lock + history authority is sufficient. If multi-replica is required: datastorage-level `AppendHistoryLineIfEventIDAbsent` under object lock (check history/sidecar, append once). Tests: concurrent ingest same event_id; retry after sidecar failure without duplicate line.
7. **Actual:** At PR head `8a808710`, in-process dedup works (`TestAppendHistoryEventIfAbsent_ConcurrentDedup`). Cross-replica duplicates possible; documented in code comment on `appendHistoryEventIfAbsent`.
8. **Reproduction:**
   1. Run concurrent unit test in `observation_history_test.go` (passes in one process).
   2. Reason about two `ObservationFunctions` values / two processes sharing one `history_object_id` without shared lock.
9. **Notes:** PR #63 review issue #7. Cross-replica duplicate append is accepted out of scope per owner decision (2026-05-23). Optional: add a one-sentence single-replica assumption to the existing observation history dedup note in `AGENTS.md`. Multi-replica fix ~120–220 LOC in datastorage + gateway only if owner reverses stance. Do not add more functions-only mutexes for cross-process safety.

## Owner decisions

- (2026-05-23) **No multi-replica:** Atlas will never run any service (including atlas-functions) as multiple replicas against shared storage. Do not design cross-replica dedup.
- In-process mutex + `history.ndjson` authority is accepted design for history dedup under ADR 0004 single-tenant deployment.

## Recommended fix

- Close as by-design for current deployment model; no datastorage-level `AppendHistoryLineIfEventIDAbsent` unless owner reverses multi-replica stance.
- Optional: brief note in `AGENTS.md` adjacent to existing observation history dedup note (single-replica assumption).
- Keep existing in-process dedup tests; do not add functions-only mutexes for cross-process safety.
