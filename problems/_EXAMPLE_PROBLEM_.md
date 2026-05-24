## Problem Template

Each entry under `problems/` is a short-lived note for agent-to-agent reference — most are resolved in minutes; none should live longer than a day or two. Use this template to keep the format consistent:

1. **Time & Date:** [UTC timestamp or local time zone timestamp]
2. **Name:** [One-line summary identifier]
3. **Issue:** [Short description of the observable problem]
4. **Severity:** [S1–S5 label from **Severity Levels** below]
5. **Location:** [Service/component and specific file/folder path associated with the issue]
6. **Expected:** [What should happen]
7. **Actual:** [What happens instead]
8. **Reproduction:** [Numbered steps, or "single command / test name" when that's enough]
9. **Notes:** [Optional — investigation hints, error snippets, links; skip if empty]

### What belongs here

- Problems hit while building, testing, or debugging — logged so the next agent session can pick up context quickly.
- Resolved or abandoned problems can stay in place as reference; no status tracking needed.

### What does not belong here

- **Recurring agent confusion** → `AGENTS.md` (after you've seen the same gotcha more than once).
- **Architectural decisions** → `docs/atlas-core/design-decisions/`.
- **How the system is supposed to work** → specs under `docs/` (e.g. `docs/atlas-core/vertical-slice-1/SPEC.md`).

### Severity Levels

- **S1 (Blocker):** Wrong data, security issue, or completely blocks the current task (dev, CI, or local stack won't run).
- **S2 (Major):** Core path broken with no reasonable workaround.
- **S3 (Moderate):** Broken edge case or painful workaround exists.
- **S4 (Minor):** Annoyance, docs drift, flaky test — task can continue.
- **S5 (Note):** Worth recording for the next agent; no real impact on the current work.

### Example

1. **Time & Date:** 2026-05-23T14:30:00Z
2. **Name:** Changefeed subscriber lag after object upload
3. **Issue:** Changefeed clients miss object-created events for large uploads
4. **Severity:** S2 (Major)
5. **Location:** `atlas-core/services/functions/internal/changefeed/hub.go`, `atlas-core/services/functions/internal/service/changefeed_server.go`
6. **Expected:** Subscribers connected before upload receive the object-created event when the upload commits
7. **Actual:** Upload completes in datastorage but subscribers only see the event after reconnecting
8. **Reproduction:**
   1. Connect a changefeed client
   2. Upload an object larger than the default chunk threshold via the functions gateway
   3. Observe missing event on the existing subscription
9. **Notes:** Check hub publish timing vs `atlas-core/services/datastorage/internal/service/grpc.go` commit order; idempotency replay or observation history append may be blocking publish.

### File naming

- Keep `_EXAMPLE_PROBLEM_.md` as the style guide; do not edit it for real incidents.
- Add one markdown file per issue, named `YYYY-MM-DD-short-slug.md` (e.g., `2026-05-23-changefeed-upload-lag.md`).
