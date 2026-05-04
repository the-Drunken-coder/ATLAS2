# 01 — Cross-layer atomicity is faked with manual rollbacks

## Fix complexity

**High.** This is the kind of thing that gets rebuilt, not patched. New table or marker file, new background worker, new tests for crash-recovery, a decision about idempotency token shape and lifetime. Realistic estimate: 1–2 weeks of focused work, plus integration tests that exercise process kills mid-operation. The architectural call (outbox vs explicit eventual consistency) is the hard part; the code is straightforward once that's chosen.

## Issue

Every object create / delete / upsert does a Postgres write and a filesystem write that can each fail independently, and the manual rollback paths between them can themselves fail — leaving permanent DB↔FS drift with no recovery scaffolding.

## In depth

`function.go:122-130, 161-176, 199-217` coordinate `postgres.ObjectStore` and `objectstorage.Store` with `rollbackObjectCreate` and `rollbackObjectUpsert`. When a step fails, the code tries to undo the previous step. When *that* undo fails, the failure is reported as a string-concatenated `OBJECT_*_ROLLBACK_ERROR` code and the system moves on.

Concrete failure modes that leave permanent drift:

- DB row written → FS folder create fails → FS rollback fails (e.g. permission glitch) → orphan DB row, no automatic recovery.
- DB row deleted → FS folder delete fails → restore-via-Upsert fails → DB row gone, files still on disk.
- Process killed (OOM, SIGKILL, container restart) between any two steps → permanent inconsistency.

There is no:

- outbox / journal table recording intent before the FS write,
- reconcile worker that retries unfinished operations on startup or on a timer,
- startup scrub that cross-checks DB rows against folders on disk,
- repair function (the spec at `SPEC.md:347-348` literally calls one out — "implementations should provide a repair/reconciliation function" — and it does not exist).

The spec also calls out a related concern explicitly for manifest writes (`SPEC.md:339`: "callers must be able to detect this partial-failure state … merely logging and continuing is not acceptable"). The Create/Delete/Upsert paths have the same partial-failure shape but have invented a different ad-hoc convention (rollback-then-error) instead of solving the durability question.

Operationally this means: a single bad-luck moment leaves an inconsistency that requires hand-cleanup via psql plus `rm -rf`. There is no mechanism for an operator or the system itself to even *notice* the inconsistency exists.

## Recommended fix

Pick one of:

1. **Outbox / journal pattern.** Add a small `pending_operations` table that records the intent of an object operation before the FS step. A background worker drains it on startup and on a timer, retrying until success. This makes operations crash-safe and idempotent.
2. **Explicit eventual consistency.** Document that DB and FS may drift, then ship the reconcile function the spec asked for (rebuild DB cache from FS, log/repair orphans on either side) and run it on startup plus on a periodic schedule.

Either way, the work includes:

- a way to mark an operation as in-flight,
- crash-recovery logic on startup,
- a reconcile/repair function with tests that *kill the process mid-call* and verify the system heals,
- idempotency tokens on Create/Upsert so client retries don't double-apply.
