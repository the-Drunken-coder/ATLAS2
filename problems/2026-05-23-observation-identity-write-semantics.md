1. **Time & Date:** 2026-05-23T00:00:00Z
2. **Name:** Observation identity writes: stale rollback and ambiguous deletion contract
3. **Issue:** (A) Live identity patches apply without recency guard: `appendIdentityPatchIfNeeded` always calls `applyIdentityPatchToObservation` after append, while replay uses `identityEffectiveAtIsNewer`—so ingest with older `observed_at` and changed identity can roll back `identity` and `latest_identity_at`. (B) Patch semantics are inconsistent: omitting `identity` is treated as deletion when `latest_telemetry` exists (`wouldRemoveIdentity` on raw patch), but `identity: null` is rejected; merge keeps `identity` on `previewJSON` while `syncObservationIdentityHistory` reads unmerged `obs.JSON` and may append `current: null`.
4. **Severity:** S1 (Blocker)
5. **Location:** `atlas-core/services/functions/internal/function/observation_history_events.go`, `observation_ingest.go`, `observation.go`, `observation_mutate.go`, `observation_json.go`, `validation.go`
6. **Expected:** Older `identity_patch` events are still appended to history but must not overwrite current `identity` / `latest_identity_at` unless `effective_at` is strictly newer than stored (`identityEffectiveAtIsNewer`). Omit `identity` in patch-style update/upsert = preserve; explicit `identity: null` = delete when telemetry exists (or reject delete without telemetry). Align `syncObservationIdentityHistory` with merged preview JSON.
7. **Actual:** Stale ingest identity overwrites row (telemetry snapshot correctly skipped—see `TestObservationFunctions_IngestObservationTelemetrySkipsStaleTelemetrySnapshot`). Omit-with-telemetry deletes identity; `identity: null` errors with `json.identity must be a JSON object`. CRUD may use client `latest_identity_at` / `started_at` as effective time and regress row.
8. **Reproduction:**
   1. Create observation with identity at T10; ingest telemetry at T6 with different `identity` payload.
   2. Observe `latest_identity_at` and `identity` regress while history line is appended.
   3. Update observation with patch `{"extra":{}}` only (no `identity` key) while row has identity + `latest_telemetry`—identity is removed.
9. **Notes:** PR #63 review. Fix ~85–200 LOC total: central guard in `appendIdentityPatchIfNeeded` (~25–40 prod) + preserve-on-omit / explicit-null contract (~110–160 with tests). Optional: server-time `effective_at` on CRUD updates. Patch-JSON section drop (issue #3) fixed at `a186054` via `previewJSON`; optional follow-up: changefeed `publishObservation` may still use unmerged `obs.JSON` when identity sync does not run (S4).

## Owner decisions

- (2026-05-23) **Identity on update:** If an update does not include `identity`, keep pre-existing identity (omit = preserve). Do not remove identity because it was omitted.
- Stale identity on live apply must match replay recency semantics (`identityEffectiveAtIsNewer`).

## Recommended fix

- Apply recency check on live identity apply in `appendIdentityPatchIfNeeded` (same guard as replay); older patches append to history but must not overwrite row `identity` / `latest_identity_at`.
- Omit `identity` on patch/update/upsert = preserve; explicit `identity: null` = clear per protocol where telemetry allows.
- Align `syncObservationIdentityHistory` with merged `previewJSON` (not raw unmerged `obs.JSON`).
- Add tests: stale ingest identity does not regress row; omit-with-telemetry preserves identity.
