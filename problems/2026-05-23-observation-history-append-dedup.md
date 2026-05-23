1. **Time & Date:** 2026-05-23T00:00:00Z
2. **Name:** History append-if-absent not safe across functions replicas
3. **Issue:** `appendHistoryEventIfAbsent` uses an in-process mutex per `historyObjectID`; concurrent same `event_id` in one process is deduped. Multiple functions instances against shared storage can both pass `historyContainsEventID` and append duplicate `history.ndjson` lines. Sidecar/index staleness is mitigated (history.ndjson fallback after `a186054`); gap tests for append-ok + sidecar-fail retry dedup and ingest-level concurrency remain.
4. **Severity:** S3 (Moderate)
5. **Location:** `atlas-core/services/functions/internal/function/observation_history_dedup.go`, `observation_history_reconcile.go`, `observation_history_events.go`; datastorage append path (no cross-process dedup)
6. **Expected:** For single-tenant single functions replica (ADR 0004 default): in-process lock + history authority is sufficient. If multi-replica is required: datastorage-level `AppendHistoryLineIfEventIDAbsent` under object lock (check history/sidecar, append once). Tests: concurrent ingest same event_id; retry after sidecar failure without duplicate line.
7. **Actual:** At PR head `8a808710`, in-process dedup works (`TestAppendHistoryEventIfAbsent_ConcurrentDedup`). Cross-replica duplicates possible; documented in code comment on `appendHistoryEventIfAbsent`.
8. **Reproduction:**
   1. Run concurrent unit test in `observation_history_test.go` (passes in one process).
   2. Reason about two `ObservationFunctions` values / two processes sharing one `history_object_id` without shared lock.
9. **Notes:** PR #63 review issue #7. Close as S5/doc-only if deployment is strictly one functions instance. Multi-replica fix ~120–220 LOC in datastorage + gateway. Do not add more functions-only mutexes for cross-process safety.
