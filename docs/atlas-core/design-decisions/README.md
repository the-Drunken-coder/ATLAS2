# Design decisions

This folder holds **architecture and product decisions** that are bigger than a single PR: tradeoffs, boundaries, and conventions we want to remember when implementing features later.

## What belongs here

- Choices that affect multiple packages or layers (API shape, persistence boundaries, retry/idempotency policy).
- Things we rejected and why (so we do not re-litigate without new facts).
- Short context: problem, decision, consequences—not full specs (those live next to the feature, e.g. `docs/atlas-core/vertical-slice-1/`).

## What does not belong here

- Living specs and checklists (use feature folders under `docs/`).
- Implementation notes that belong next to code (`AGENTS.md`, package comments).

## File naming

Use numbered records so order stays clear:

`NNNN-short-kebab-title.md`

Example: `0001-api-boundary-idempotency-versioning.md`

Bump `NNNN` for each new decision. Numbers are chronological, not priority.

## Suggested structure for each record

1. **Status** — Proposed | Accepted | Superseded by NNNN
2. **Context** — What forced the decision.
3. **Decision** — What we agreed to do.
4. **Consequences** — Tradeoffs, follow-ups, optional alternatives.

Keep each file readable in a few minutes.
