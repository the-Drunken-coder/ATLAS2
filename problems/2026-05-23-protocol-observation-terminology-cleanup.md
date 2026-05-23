1. **Time & Date:** 2026-05-23T00:00:00Z
2. **Name:** Residual sighting terminology in protocol validators and track examples
3. **Issue:** PR #63 resolved most P3 review nits (bearing-only observation label, TS `validateTelemetryEnvelope`, docs `history.*` paths, schema bundles in sync). Remaining cosmetic drift: Go/TS `validateTelemetryEnvelope` parameter still named `sighting`; track example `operator_notes` says "multiple sightings" in mirrored example JSON files.
4. **Severity:** S5 (Note)
5. **Location:** `atlas-protocol/packages/go/custom_rules.go`, `atlas-protocol/packages/typescript/src/validator.ts`, `atlas-protocol/examples/tracks.json`, `docs/atlas-protocol/examples/tracks.json`
6. **Expected:** Telemetry helpers use `telemetry` naming; track examples say observations/telemetry where appropriate. Intentional `latest_sighting` in forbidden-key docs unchanged.
7. **Actual:** Parameter `sighting` in both validators; track operator note still uses sightings wording.
8. **Reproduction:**
   1. `rg 'func validateTelemetryEnvelope\(sighting' atlas-protocol/`
   2. `rg 'multiple sightings' atlas-protocol/ docs/atlas-protocol/`
9. **Notes:** PR #63 review issue #11 at `8a808710`. ~12 LOC, no schema regen. Adjacent gap (not P3): no TS `ValidateObservationHistoryEvent`—tracked in `2026-05-23-observation-history-event-validator-parity.md`.

## Owner decisions

- (2026-05-23) Cosmetic terminology cleanup only; no behavior or schema changes. Leave intentional `latest_sighting` forbidden-key docs unchanged.

## Recommended fix

- Rename `validateTelemetryEnvelope` parameter `sighting` → `telemetry` in Go and TypeScript.
- Update track example `operator_notes` ("multiple sightings" → observations/telemetry wording) in `atlas-protocol/examples/tracks.json` and `docs/atlas-protocol/examples/tracks.json`.
- No schema regen; history validator parity remains in `2026-05-23-observation-history-event-validator-parity.md`.
