# Atlas Protocol Contracts

This folder contains the protocol-owned document contracts.

## Contracts

- [Resources](resources.md): entity, task, observation, and object JSON.
- [Command catalog](command-catalog.md): command definitions and parameter
  schema shape.
- [Validation errors](errors.md): stable error shape and initial error codes.
- [Change events](change-events.md): deferred event-shape questions.

## Contract Rules

Contracts should describe Atlas-shaped data, not implementation plumbing.

Put a rule here when it can be checked without querying Atlas Core state.

Examples of protocol rules:

- `track` telemetry requires latitude and longitude together.
- command catalog entries have unique command IDs.
- top-level promoted fields are forbidden inside caller-owned JSON.

Keep runtime rules in Atlas Core docs.

Examples of Core rules:

- the target asset exists
- the asset supports the requested command
- the stored command catalog exists
- object manifest cache writes follow Core filesystem rules
