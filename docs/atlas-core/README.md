# Atlas Core Docs

This folder contains docs for Atlas Core, the Go service implementation that
stores, validates, indexes, and serves Atlas data locally.

Atlas Core docs are implementation-focused. They describe storage, stores,
function-layer behavior, local stack tooling, and how Atlas Core should consume
or enforce Atlas data contracts.

## Contents

- `vertical-slice-1/`: storage, stores, function-layer foundation, local stack,
  and runtime basics.
- `vertical-slice-2/`: Atlas Core integration with Atlas Protocol validation,
  including function-layer placement, runtime-check boundaries, error mapping,
  and verification criteria.
- `design-decisions/`: Atlas Core architecture decisions that are broader than
  one code change.
- `evaluation/`: Atlas Core quality and remediation notes.

Protocol-level docs live separately under `docs/atlas-protocol/`.
