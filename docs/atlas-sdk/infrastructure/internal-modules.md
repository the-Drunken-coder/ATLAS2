# Internal Modules

Likely internal modules:

- `client`: `AtlasClient` construction and namespace wiring.
- `http`: fetch wrapper, request construction, response parsing.
- `errors`: SDK error classes and error conversion.
- `service`: health, readiness, service metadata.
- `entities`: entity resource client.
- `objects`: object metadata and object file clients.
- `tasks`: task resource client.
- `observations`: observation resource client.
- `sync`: server-filtered subscriptions, service events, local cache, and
  refresh.
- `types`: public TypeScript types.

Internal modules should keep resource behavior separate from shared transport
and error handling.
