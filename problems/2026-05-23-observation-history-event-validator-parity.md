1. **Time & Date:** 2026-05-23T00:00:00Z
2. **Name:** observation_history_event validator paths and TS parity incomplete
3. **Issue:** JSON Schema and Go custom rules largely agree on telemetry (`observed_at`), identity_patch/lifecycle (`effective_at`, forbid `observed_at`). Go schema validation emits `json.*` field paths while custom rules and `docs/atlas-protocol/contracts/errors.md` use `history.*`—clients see inconsistent paths; duplicate required issues possible. TypeScript reference validator has no `ValidateObservationHistoryEvent`. Few goldens/tests for invalid history envelope shapes.
4. **Severity:** S4 (Minor)
5. **Location:** `atlas-protocol/source/schemas/observation_history_event.schema.json`, `atlas-protocol/packages/go/custom_rules.go`, `atlas-protocol/packages/go/schema_errors.go`, `atlas-protocol/packages/typescript/src/validator.ts`, `docs/atlas-protocol/contracts/errors.md`, `atlas-core/services/shared/protocolvalidation/validator_test.go`
6. **Expected:** Schema, Go, TS, and docs agree on required fields per `event_type` and stable `history.*` issue paths. TS can validate history lines for SDK/CLI parity. Goldens cover invalid telemetry/identity_patch cases.
7. **Actual:** Core append uses Go validator (OK for production). Schema-driven errors like `json.effective_at` vs `history.effective_at`. No TS history validator; thin `custom_rules_test.go` coverage; no history lines in `protocolvalidation/validator_test.go`.
8. **Reproduction:**
   1. Call Go `ValidateObservationHistoryEvent` with missing `effective_at` on identity_patch—inspect issue `field` prefix.
   2. Search `validator.ts` for `observation_history_event` / `ValidateObservationHistoryEvent` (absent).
9. **Notes:** PR #63 P2 review issue #8. Fix ~30 LOC Go (`prefixIssues(schemaIssues, "history")`) + ~80–150 TS + ~60–90 goldens/tests. Lifecycle events not emitted by Core yet—schema optional `effective_at` is fine. Identity payload not structurally validated in Go beyond envelope.

## Owner decisions

- (2026-05-23) Validator parity across Go, TypeScript, schema, and `errors.md` is required for SDK/CLI consumers; stable `history.*` issue paths are the contract.

## Recommended fix

- Align Go schema error paths to `history.*` prefix (e.g. `prefixIssues(schemaIssues, "history")`); ensure custom rules match `docs/atlas-protocol/contracts/errors.md`.
- Add `ValidateObservationHistoryEvent` in TypeScript `validator.ts` for parity with Go.
- Add goldens/tests for invalid telemetry and identity_patch envelope shapes; extend `protocolvalidation/validator_test.go` with history line cases.
