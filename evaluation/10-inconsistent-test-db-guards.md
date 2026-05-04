# 10 — Inconsistent test-DB safety guards

## Fix complexity

**Low.** A few hours. Mostly mechanical.

## Issue

Two packages each implement a "is this safe to wipe?" check on the test database, and they use different rules and different override env vars — meaning a database name that one package considers safe to truncate, the other refuses, and vice versa.

## In depth

`postgres/testutil_test.go:29-32`:

```go
hasTestSuffix := strings.HasSuffix(cfg.PostgresDB, "_test")
if !hasTestSuffix && !allowCleanup {
    t.Fatalf("refusing to run cleanup on database %q: ...
        database name must end with '_test' or set ATLAS_ALLOW_DB_CLEANUP=true", cfg.PostgresDB)
}
```

`function/function_integration_test.go:46-50`:

```go
allowRealDB := os.Getenv("ATLAS_ALLOW_REAL_DB_OVERWRITE") == "true"
isTestDB := strings.Contains(strings.ToLower(cfg.PostgresDB), "test")
if !isTestDB && !allowRealDB {
    t.Fatalf("refusing to run destructive tests on database %q: ...
        database name must contain 'test' or set ATLAS_ALLOW_REAL_DB_OVERWRITE=true", cfg.PostgresDB)
}
```

Differences:

- **Rule:** `HasSuffix("_test")` vs `Contains("test")`. The second is strictly looser — `atlas_test_safe` passes `Contains` but not `HasSuffix`. So one package will run, the other won't, on the same DB name.
- **Override env var:** `ATLAS_ALLOW_DB_CLEANUP` vs `ATLAS_ALLOW_REAL_DB_OVERWRITE`. To run both packages against the same DB you need both vars set; nobody documents this.
- **`envOrDefault`** is also re-implemented in `testutil_test.go:67-72`, copy-pasted from `config/config.go:69`. Two-line function but it's a small symptom of "no shared test helper module."

This isn't load-bearing today (only the team runs it) but it's the kind of inconsistency that bites when CI is wired up and one suite passes while the other refuses.

## Recommended fix

1. Move the safety guard into a shared `internal/testsupport` (or similar) package.
2. Pick one rule. `HasSuffix("_test")` is the stricter and more conventional choice.
3. Pick one override env var. `ATLAS_ALLOW_DB_CLEANUP` is fine.
4. Move `envOrDefault` into the same shared package (or just promote `config.envOrDefault` to a small public helper).
5. Document the policy in one place (the test support package's doc.go).
