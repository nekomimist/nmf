# NMF Architecture Review (Active)

Date: February 6, 2026 (updated: February 7, 2026)
Author: Codex (implementation-assisted review)

## Purpose

This document tracks only unresolved architecture and reliability risks.
Completed items are intentionally removed to keep this file actionable for upcoming work.

## Current Status

- Phase 1 reliability work is complete as of February 7, 2026.
- Watcher lifecycle hardening, jobs subscription lifecycle, VFS path validation alignment, and the major `main.go` split are done.
- Detailed implementation history is preserved in git history and related commits.

## Active Findings

### 1) Medium: Non-Linux direct SMB provider path for jobs is not implemented

Evidence:

- `internal/fileinfo` direct SMB provider currently targets Linux implementation paths.
- `internal/jobs` SMB job execution depends on provider capability.

Risk:

- Behavior parity for `smb://` copy/move can differ across platforms when CIFS mount fallback is unavailable.

Recommended follow-up:

- Define and implement provider capability parity for Windows/non-Linux targets (or explicitly constrain supported execution paths per OS).
- Add platform-specific integration tests for the selected behavior.

Acceptance criteria:

- `smb://` copy/move behavior is explicitly documented per OS.
- Non-Linux behavior either matches Linux capability or has tested, documented fallback semantics.

### 2) Medium: SMB integration tests are not wired into CI

Evidence:

- SMB integration flow is manually executable (`NMF_SMB_TEST_DIR=...`) but not part of repository CI.

Risk:

- Regressions in network-path copy/move can ship undetected.

Recommended follow-up:

- Add CI workflow with stable SMB fixture (for example, dockerized Samba).
- Gate SMB integration jobs appropriately (nightly or labeled PR runs if runtime is high).

Acceptance criteria:

- CI runs SMB integration tests on a repeatable fixture.
- Failure signals are visible and actionable in PR checks.

## Verification Baseline

Use these checks before and after architecture-affecting changes:

- `GOCACHE=/home/neko/src/nmf/.gocache go test ./...`
- `GOCACHE=/home/neko/src/nmf/.gocache go test -race ./internal/watcher ./internal/jobs`
- `NMF_SMB_TEST_DIR=smb://host/share/path GOCACHE=/home/neko/src/nmf/.gocache go test ./internal/jobs -run TestSMBCopyRoundtrip`

## Review Trigger

Update this file when:

- A new unresolved architecture risk is found.
- An active finding is completed (remove it from this file).
- Scope or platform support policy changes for VFS/SMB behavior.
