# SDK Method Contract

## Purpose

This folder defines the Atlas SDK method families before Atlas Core freezes the
public API route contract.

The SDK should describe what client code needs to do. The API should then be
built as the bridge that serves those SDK methods through the Core function
layer.

This is not a route list.

## Contract Level

Each method-contract file should define:

- the SDK method family
- the caller intent it serves
- the minimum input shape the SDK should accept
- the output shape callers should expect
- whether the method uses direct request mode or subscribed local cache state
- the API capabilities required to support the method
- known Core support or gaps

Avoid concrete HTTP paths in this folder. Route names belong later, after the
method shape is clear.

## Files

- `service.md`: service status and metadata methods.
- `entities.md`: entity resource methods.
- `objects.md`: object info and content methods.
- `tasks.md`: task record methods.
- `observations.md`: observation record methods and query intents.
- `sync.md`: subscriptions, events, local cache, and refresh methods.

## Naming Sketch

Use a single SDK client object with resource groups:

- `atlas.service.*`
- `atlas.entities.*`
- `atlas.objects.*`
- `atlas.tasks.*`
- `atlas.observations.*`
- `atlas.sync.*`

The final TypeScript names can change during implementation, but the resource
grouping should stay stable unless the Core resource model changes.

## Method Shape Rule

Keep the exposed SDK surface small.

Resource query variants should usually be options on `list(...)`, not separate
top-level methods. Subscription variants should be options or scope values on
`sync.subscribe(...)`, not separate subscribe methods.

Use separate SDK methods only when the caller intent is genuinely different.
For example, object info and object content are separate method families because
one returns metadata and JSON while the other returns file bytes or parsed
content.

## Source Relationship

Use `../features/` for product-level feature intent.

Use this folder for the method families that drive API planning.

Use the future API contract for exact transport details, status codes, headers,
request envelopes, and response envelopes.
