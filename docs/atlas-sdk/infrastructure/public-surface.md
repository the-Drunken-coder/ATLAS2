# Public Surface

## Exports

The SDK should expose a small public surface:

- `AtlasClient`
- client configuration types
- resource request/response types
- event types
- SDK error types
- helper types for file upload/download

Avoid exporting internal HTTP helpers, route builders, cache internals, or
transport implementation details.

## Client Shape

Use one primary client:

```ts
const atlas = new AtlasClient({
  baseUrl: "http://localhost:8080",
});
```

Group resource methods under namespaces:

```ts
atlas.service.ready()
atlas.entities.list()
atlas.objects.get("object-001")
atlas.tasks.create(...)
atlas.observations.create(...)
atlas.sync.subscribe(...)
```
