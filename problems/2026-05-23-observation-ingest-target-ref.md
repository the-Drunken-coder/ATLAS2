1. **Time & Date:** 2026-05-23T00:00:00Z
2. **Name:** Telemetry ingest accepts target_entity_id that is never persisted on existing rows
3. **Issue:** `validateIngestMatchesObservation` only rejects `target_entity_id` mismatch when both stored and incoming targets are non-nil. When stored target is nil and ingest sends a non-nil target, validation passes but the existing-row path never assigns `ingest.TargetEntityID` to the observation—silent no-op.
4. **Severity:** S2 (Major)
5. **Location:** `atlas-core/services/functions/internal/function/observation.go` (`IngestObservationTelemetry`, `validateIngestMatchesObservation`, `executeIngestTelemetry`); `observation_history_reconcile.go` (`applyIngestLifecycleOverlay`)
6. **Expected:** Contract A (recommended): refs on an existing observation are immutable via ingest—any incoming `target_entity_id` differing from stored (including nil→non-nil) is rejected with a clear error including `observation_id`; first binding uses create path or `UpdateObservation`. `source_asset_id` mismatch remains rejected (already implemented).
7. **Actual:** Nil stored + non-nil incoming target succeeds HTTP/gRPC but row `target_entity_id` stays NULL. `applyIngestLifecycleOverlay` would copy target on reconcile but normal ingest never puts ingest target on `overlay`.
8. **Reproduction:**
   1. Create or load observation with `target_entity_id` NULL.
   2. `IngestObservationTelemetry` with same `observation_id` / `source_asset_id` and non-nil `target_entity_id`.
   3. Reload row—target still NULL.
9. **Notes:** PR #63 review (`a186054` added partial validation). Fix ~25–40 LOC prod + ~70–100 LOC tests. Do not half-implement adopt-on-ingest (Contract B) without persist + ref validation + reconcile alignment. Plan doc allows target changes via Update, not ingest late-bind.

## Owner decisions

- (2026-05-23) **Ingest target (Contract A):** Ingest must NOT late-bind `target_entity_id`. Setting or changing target is via `UpdateObservation` only.
- If ingest `target_entity_id` does not match the row (including nil→non-nil), reject with a clear error—no silent no-op.

## Recommended fix

- Extend `validateIngestMatchesObservation` to reject any `target_entity_id` mismatch vs stored row, including NULL stored + non-nil incoming.
- Do not persist target on ingest for existing rows; do not implement Contract B (adopt-on-ingest) without owner approval.
- Document Contract A in `resources.md` or adjacent ingest docs if not already explicit.
- Add tests: nil stored + non-nil ingest target fails with clear error including `observation_id`.
