# Build And Package

## Package Direction

Build one TypeScript/npm package.

Do not publish externally as part of the first planning slice. Local workspace
use is enough until the API and SDK surface settle.

## Tooling Direction

Keep tooling simple:

- TypeScript strict mode
- npm or pnpm scripts
- Vitest or equivalent lightweight test runner
- small bundler only if needed for package output
- no heavy framework

The old npm helper used TypeScript, tsup, and Vitest. That is a reasonable
reference point, but the exact tool choices can wait until implementation.

## Runtime Target

Target modern runtimes with fetch:

- modern Node
- browser-like runtimes
- test runtimes with injected fetch

Do not add legacy runtime support unless a real consumer requires it.

## Versioning

Atlas SDK and Atlas Core should target the current system version together.

Do not add API version negotiation, compatibility matrices, or multi-version
support in this phase.

