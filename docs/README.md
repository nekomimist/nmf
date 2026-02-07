# Documentation Guide

## Intent

This directory is the canonical source for project design and operational documentation.

## What goes where

- `AGENTS.md`
  - Agent execution guidelines and collaboration rules.
  - Keep this concise and task-execution oriented.
  - Avoid storing long-lived architecture rationale here.
- `docs/architecture/`
  - Stable technical design and module boundaries.
  - Cross-cutting behavior (VFS/SMB, watcher lifecycle, UI input model).
  - Current index: `docs/architecture/README.md`.
- `docs/architecture-review.md`
  - Active architecture risk register (unresolved items only).
- `docs/runbooks/` (recommended when needed)
  - Operational procedures and troubleshooting steps.
- `docs/adr/` (recommended when needed)
  - Architecture Decision Records (one file per decision).

## Maintenance policy

- If a topic explains "how the system should be designed," place it in `docs/architecture/`.
- If a topic explains "how agents should operate in this repo," place it in `AGENTS.md`.
- If a risk is resolved, remove it from `docs/architecture-review.md`.
- Prefer short, linkable pages over one large document.
