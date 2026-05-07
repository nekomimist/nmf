# Architecture Docs

## Scope

This folder holds durable design documentation.

## Pages

- `overview.md`
  - Runtime composition, package boundaries, and key data flows.
- `vfs-smb.md`
  - Canonical path model and provider-specific behavior.
- `platform-behavior.md`
  - Platform-specific desktop integrations and support policy.
- `watcher-jobs.md`
  - Watcher lifecycle contract and jobs manager integration points.
- `ui-input.md`
  - KeyManager stack, focus model, and dialog input conventions.
- `../starlark-configuration.md`
  - User-facing Starlark overlay syntax and custom command behavior.

## Writing rules

- Prefer behavior contracts over implementation narration.
- Include concrete file references for critical invariants.
- Update docs in the same PR when architecture behavior changes.
