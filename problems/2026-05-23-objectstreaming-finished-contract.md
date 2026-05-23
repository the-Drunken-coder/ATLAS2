1. **Time & Date:** 2026-05-23T00:00:00Z
2. **Name:** objectstreaming WriteChunkSink finished flag ignored; double trailing recv
3. **Issue:** `ProcessWriteChunks` / `ProcessAppendChunks` assign `finished` from sinks then discard it (`_ = finished`) and always perform another post-final `recv()`. Forward sinks (`NewForwardWriteSink`, `NewForwardAppendSink`) drain `recv` inside the sink and return `finished=true`, so production performs two trailing receives on the same stream. Type comments contradict behavior; one test name implies `finished=true` but uses `finished=false`.
4. **Severity:** S3 (Moderate)
5. **Location:** `atlas-core/services/shared/objectstreaming/process.go`, `upload_sink.go`, `atlas-core/services/functions/internal/service/object_streaming.go`; tests `process_test.go`, `upload_sink_test.go`
6. **Expected:** Single owner of trailing stream drain and EOF validation—either sinks never recv (Option A: `Process*` owns drain) or `Process*` skips recv when `finished=true` (Option B with documented invariant). Tests lock one post-final recv on forward path and late-chunk-after-final rejection.
7. **Actual:** Double recv on happy path (likely OK on gRPC). `datastorage` writer sinks return `finished=false`—unaffected. Regression: PR #80 dropped `if finished { return nil }` from PR #65.
8. **Reproduction:**
   1. Trace `processForwardWriteChunks` → `ProcessWriteChunks` + `NewForwardWriteSink`.
   2. Read `process.go` after `final` chunk—note `_ = finished`.
   3. Run `TestProcessWriteChunksRejectsChunksAfterFinalWhenSinkReturnsFinished` and inspect sink return value.
9. **Notes:** PR #63 review. Low production risk today; high maintainability debt. Fix Option A ~50–90 LOC (remove `finished`, sinks don't drain); Option B ~10–20 LOC restore `if finished { return nil }` only if sink guarantees prior drain.

## Owner decisions

- (2026-05-23) No product constraint beyond existing problem scope; fix is internal API / maintainability only.

## Recommended fix

- **Option A:** Remove reliance on `finished` flag; `ProcessWriteChunks` / `ProcessAppendChunks` own a single trailing `recv` after the sink returns.
- Forward sinks (`NewForwardWriteSink`, `NewForwardAppendSink`) must not drain the stream; align sink contract with `Process*`.
- Update tests: one post-final recv on forward path; late-chunk-after-final rejection.
- Scope: internal API only (`objectstreaming`, functions forwarding)—no external contract change.
