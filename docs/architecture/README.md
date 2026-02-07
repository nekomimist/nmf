# Architecture Docs

## Scope

This folder holds durable design documentation.

## Pages

- `overview.md`
  - Runtime composition, package boundaries, and key data flows.
- `vfs-smb.md`
  - Canonical path model and provider-specific behavior.
- `watcher-jobs.md`
  - Watcher lifecycle contract and jobs manager integration points.
- `ui-input.md`
  - KeyManager stack, focus model, and dialog input conventions.

## Writing rules

- Prefer behavior contracts over implementation narration.
- Include concrete file references for critical invariants.
- Update docs in the same PR when architecture behavior changes.
