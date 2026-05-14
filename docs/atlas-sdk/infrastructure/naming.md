# Naming

## Recommendation

Use **Atlas SDK** as the docs and product name.

Possible package names can be decided later, but likely candidates are:

- `@atlas/sdk`
- `@atlas/atlas-sdk`
- `@atlasnpm/atlas-sdk`

## Why Not "Connection Package"

"Connection package" is accurate but awkward. It describes implementation
plumbing rather than the user-facing role.

The package should be the normal way developers interact with Atlas Core, not
just a thin connection helper.

## Why "SDK"

"SDK" fits the intended scope:

- typed client for Core API calls
- file helpers
- event subscription helpers
- typed errors
- optional sync/cache helpers

It also matches the Atlas C3 docs, which already describe the developer-facing
package as Atlas SDK.

## Naming Rule

Use "Atlas SDK" in planning docs unless the user chooses a different name.

Avoid committing to the final npm package name until package structure and
publishing scope are settled.
