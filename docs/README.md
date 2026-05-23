# ATLAS2 Docs

Project docs are split by ownership:

- `atlas-core/`: implementation docs for the Atlas Core Go service.
- `atlas-protocol/`: protocol-level docs for the shared Atlas data contract.
  Start with `atlas-protocol/README.md`, then read `atlas-protocol/contracts/`.
- `atlas-sdk/`: deferred planning for the future TypeScript client package;
  start with `atlas-sdk/design-principles.md` (not method-level contracts).

Use `atlas-protocol/` when the question is "what is valid Atlas data?"

Use `atlas-core/` when the question is "how does the Atlas Core service store,
validate, or operate on that data?"

Use `atlas-sdk/` when the question is "how should client code interact with
Atlas Core?"
