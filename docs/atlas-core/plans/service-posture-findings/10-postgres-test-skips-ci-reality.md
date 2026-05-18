# Postgres Test Skips Versus CI Reality

Date evaluated: 2026-05-18

Scope: current `feature/datastorage-internal-auth-boundary` working tree under
`atlas-core/services`, `atlas-core/proto`, and `docs/atlas-core`.

## Judgment

Real local-developer footgun, mitigated by CI. Low to medium severity.

## Evidence

- `atlas-core/services/datastorage/internal/postgres/testutil_test.go:55` through
  `atlas-core/services/datastorage/internal/postgres/testutil_test.go:58` skip
  Postgres-backed package tests when Postgres is not reachable.
- `.github/workflows/ci.yml:75` through `.github/workflows/ci.yml:100` provision a
  Postgres service for CI.
- `.github/workflows/ci.yml:115` and `.github/workflows/ci.yml:116` run
  `go test -p 1 ./...` in CI.
- `.github/workflows/integration.yml:27` through `.github/workflows/integration.yml:41`
  build/start the Docker stack and run integration tests with
  `-tags=integration`.

## Reasoning

The repo is not blind in PR CI. The skip behavior mainly affects local runs where
a developer may see a green package set without realizing important Postgres
coverage did not execute.

## Best Fix

Document the behavior in contributor setup. Optionally add
`ATLAS_ENFORCE_POSTGRES_TESTS=1`: when set, Postgres connection failure should
fail instead of skip. CI can set it to ensure the contract remains explicit, even
though CI already provisions Postgres.
