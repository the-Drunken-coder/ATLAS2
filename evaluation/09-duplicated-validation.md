# 09 — Validation duplicated 2–3× across layers

## Fix complexity

**Low.** Half a day to refactor and re-run tests. The risk is removing a check at one layer and discovering a caller that depended on it — straightforward to catch with the existing test suite if the audit is done carefully.

## Issue

`ValidateObjectID` runs at the function layer, inside `ValidateSafeObjectPath`, and again inside every internal `objectPath` / `filePath` / `manifestPath` call — a single `WriteObjectFile` validates the same ID three or more times, and it's unclear which layer is supposed to own the rule.

## In depth

Trace a `WriteObjectFile("foo", "bar.txt", data)` call through the code:

- `function.WriteFile` calls `objStore.ValidateSafeObjectPath` (`function.go:270`) — runs `ValidateObjectID` plus filename checks.
- `objStore.WriteObjectFile` calls `s.ValidateSafeObjectPath` again at `store.go:89`.
- `objStore.WriteObjectFile` calls `s.filePath` at `store.go:92` — which calls `ValidateSafeObjectPath` *again* at `store.go:273`.
- `s.filePath` calls `safeJoinUnderRoot` which does its own resolution checks.

For a Create flow it's even worse: `validateObjectModel` (`function.go:497`) also calls `ValidateObjectID` directly.

The redundant work is microseconds, so the cost isn't performance — it's confusion. When a future change tightens a rule (say, lowercase-only IDs), there are 4–5 places to change in lockstep, and forgetting one creates a check that's strict at one layer and loose at another. That's exactly the shape of bug that produces inconsistent error messages in production.

The deeper question: **which layer owns this validation?** Right now nobody does — every layer redoes it defensively. The function layer's `validateObjectModel` is the natural owner for *user-facing* rules (object_id required, type required, valid charset). The objectstorage layer's `ValidateSafeObjectPath` is the natural owner for *path-safety* rules (no `..`, no separators, not the reserved manifest filename). Conflating them means they get repeated together everywhere either is needed.

## Recommended fix

1. Decide which layer owns each class of rule. Suggested split:
   - **Function layer:** business rules (required, length, charset).
   - **Objectstorage layer:** path-safety rules (no `..`, no separators, reserved names).
2. Have `ValidateSafeObjectPath` only do path-safety, not delegate to `ValidateObjectID`'s charset rule.
3. Trust the boundary: internal helpers (`objectPath`, `filePath`, `manifestPath`) should *assume* a validated input — document with a comment, don't re-validate. Make them unexported (they already are) and treat the package surface as the validation boundary.
4. Add a single test per validation rule at the layer that owns it, instead of testing every rule everywhere it's invoked.
