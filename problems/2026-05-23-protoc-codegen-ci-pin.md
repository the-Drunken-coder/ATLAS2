1. **Time & Date:** 2026-05-23T00:00:00Z
2. **Name:** CI protoc version must match committed gRPC stubs (PR #63)
3. **Issue:** `main` CI used Ubuntu apt `protobuf-compiler` (3.x) while PR regenerated `atlas-core/services/shared/gen` with protoc 34.x (`v7.34.1` in file headers)—`python3 atlas.py codegen-check` failed on header-only diffs until workflow pinned official protoc v34.1.
4. **Severity:** S5 (Note)
5. **Location:** `.github/workflows/ci.yml`, `atlas-core/services/shared/gen/**/*.pb.go`, repo-root `atlas.py` (`codegen-check`)
6. **Expected:** CI installs protoc v34.1; `codegen-check` passes without dirty `gen/`. Contributors understand v34.1 release tag vs `protoc v7.34.1` header string are the same compiler.
7. **Actual:** Fixed on PR branch (`b074cc7` workflow pin); Contracts job green at `8a808710`. `main` still apt until merge.
8. **Reproduction:**
   1. On branch without workflow pin: run `python3 atlas.py codegen-check` with apt protoc 3.x vs committed stubs.
   2. On PR branch: CI Contracts step with pinned zip.
9. **Notes:** Do not regenerate stubs only to change header version strings. Regenerate when `.proto` changes. Optional: one-line note in `AGENTS.md` for v34.1 / v7.34.1 mapping. Large `gen/` diff on PR is mostly real RPC changes (`IngestObservationTelemetry`), not version confusion.

## Owner decisions

- (2026-05-23) CI protoc version must match local/codegen and committed `atlas-core/services/shared/gen` stubs; apt `protobuf-compiler` 3.x is not acceptable for `codegen-check`.

## Recommended fix

- Pin protoc v34.1 in `.github/workflows/ci.yml` (official zip), matching PR branch fix.
- Document v34.1 release tag vs `protoc v7.34.1` header string mapping in `AGENTS.md` if not already present.
- Do not regenerate stubs solely to change header version strings; regenerate only when `.proto` changes.
