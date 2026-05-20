# Features

**Not a maintained contract.** SDK work is deferred. Durable client-facing
principles live in [../design-principles.md](../design-principles.md). Files here
are exploratory notes only and may name methods, scopes, or behaviors that are
not decided or that no longer match Core.

Each file below sketches one SDK feature family for early thinking. Shared
package mechanics belong in `../infrastructure/`.

## Files

- `service.md`: health, readiness, and service metadata.
- `entities.md`: entity resource methods.
- `objects.md`: object metadata and object file methods.
- `tasks.md`: task resource methods.
- `observations.md`: observation resource methods.
- `sync.md`: server-filtered subscriptions, events, local cache, refresh, and
  broad current-state sync.
