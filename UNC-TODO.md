# UNC Support Plan (Windows & Linux)

This document tracks UNC/SMB support across Windows and Linux. It now reflects what is implemented vs what remains, with the goal of keeping UI code stable via a small VFS and resolver.

## Status Snapshot
- Completed:
  - Resolver & normalization: Windows UNC ⇔ `smb://` conversion; Linux `smb://`/`//` mount detection via `/proc/self/mountinfo` and fallback to SMB client.
  - Minimal VFS (`ReadDir`) and portable read path (`ReadDirPortable`).
  - Windows: UNC access retry by establishing a temporary session via `WNetAddConnection2W` on access errors (using keyring/UI creds).
  - Linux: Direct SMB provider using `github.com/hirochachacha/go-smb2` (read-only listing) and async credential prompting.
  - Credentials UI dialog, in-memory cache, and OS keyring integration (99designs/keyring).
  - Path input normalization; SMB navigation uses canonical `smb://` for display/history.
  - Directory watcher uses portable listing and lengthens poll interval for `smb://` paths.
  - Windows icon service with async fetch and batching.

- Remaining (high level):
  - Expand VFS to include `Stat/Open/Join/Base` and remove remaining `filepath.*` for SMB paths in UI.
  - Tree dialog to use portable VFS (`ReadDirPortable`) instead of direct `os.ReadDir`/`os.Stat`.
  - Network/auth error typing + friendly UI messages; add conservative retry/backoff for transient SMB errors.
- Credentials precedence: URL → memory → keyring → UI [done].
  - Watcher list source injection and capability-based tuning instead of string heuristics.
  - Unit tests for resolver normalization, mount detection (Linux), and Windows UNC⇔`smb://` round-trip.
  - Windows long-path (`\\?\UNC\`) policy for edge cases.

## Goals
- Windows: Handle `\\server\share\path` and optionally `\\?\UNC\server\share\path` via native calls.
- Linux: Treat `smb://server/share/path` (and `//server/share/path`) as SMB; prefer existing mounts, else direct SMB.
- Keep UI, history, watcher, and icon pipelines unchanged from the caller’s perspective via a small VFS.

## Non-Goals (Initial Phase)
- Network browsing and share enumeration.
- Copy/move over SMB; focus on listing/metadata.
- Kernel notifications on SMB; continue polling for remote.

## Architecture: VFS Abstraction
Minimal VFS is in place for directory listing; future work extends it.

- Implemented now:
  - `type VFS interface { ReadDir(path string) ([]os.DirEntry, error); Capabilities() Capabilities }`
  - `type Capabilities struct { FastList bool; Watch bool }`
  - Providers implement `ReadDir`; higher-level code calls `ReadDirPortable`.

- Remaining:
  - Add `Stat`, `Open`, `Join`, `Base` to VFS and migrate UI helpers (`smbJoin/smbParent`) and `filepath.*` usage to provider-aware APIs.

## Resolver
- Implemented:
  - Windows: Accept UNC; convert `smb://` to UNC internally; expose display as `smb://`.
  - Linux: For `smb://`/`//`, detect CIFS/SMB mounts from `/proc/self/mountinfo`; otherwise fall back to SMB provider. Display uses `smb://`.

## Providers
- LocalFS (all OS): Standard library; on Linux also used for existing CIFS mounts.
- SMBFS (Linux): `go-smb2` backed; directory listing implemented; capabilities mark remote (no watch).

## Path Model & Normalization
- Display string is canonical `smb://server/share/path` (used for history/UI).
- Provider-native strings:
  - Windows LocalFS: UNC (`\\host\share\...`, `\\?\UNC\...` for long paths in future).
  - Linux LocalFS: local absolute mount path.
  - Linux SMBFS: host/share + relative segments handled by provider.
- Remaining: Replace residual `filepath.*` in UI when path is `smb://` with VFS `Join/Base` once added.

## Credentials & Auth Flow
- Implemented:
  - CredentialsProvider (UI dialog) wrapped with in-memory cache; OS keyring integration via `internal/secret`.
  - Windows UNC connect helper uses cached/keyring creds and persists to keyring on success when requested.
- Remaining:
  - Enforce precedence clearly: seed from URL into memory cache, then prefer memory → keyring → UI provider.

## Directory Watching
- Implemented:
  - Uses portable listing; longer poll interval for `smb://` paths.
- Remaining:
  - Accept injectable list source and use `Capabilities` to tune rather than string prefix checks.

## UI/UX Notes
- Implemented: Path entry accepts local, UNC (Windows), and `smb://`; display/history normalized to `smb://`.
- Remaining: Error messages with specific categories (auth required/failed, unreachable, share not found, timeout).

## Error Types & Retry
- Remaining:
  - Add network-aware error wrappers under `internal/errors` (auth required/failed, host unreachable, share not found, timeout).
  - Add conservative retries/backoff for transient network errors; prompt on auth failures.

## Testing Strategy
- Remaining:
  - Table-driven tests for resolver & normalization (Windows UNC ⇔ `smb://`, edge cases).
  - Mount detection parsing (`/proc/self/mountinfo`) tests on Linux.
  - FS mocking for watcher flows; skip live SMB in CI.

## Rollout Roadmap (Status)
- [x] 1) Introduce VFS (minimal)/Capabilities/Resolver/Parsed; call sites use portable read.
- [x] 2) Validate Windows via LocalFS (UNC end-to-end) with connection helper on access errors.
- [x] 3) Linux mount detection and LocalFS mapping.
- [x] 4) Add SMBFS provider and CredentialsProvider + login dialog (Linux first).
- [~] 5) Make watcher VFS-aware; currently uses portable read + interval tweak. Inject list source and use Capabilities next.
- [~] 6) Normalize history/config to `smb://` (done); add unit tests (pending).

## Touch Points
- `internal/fileinfo`: VFS (`vfs.go`), resolver (`resolver.go`), SMB providers (`smbfs_*.go`), Windows UNC connect helper.
- `internal/ui`: SMB login dialog; path entry normalization is wired in `main.go`.
- `internal/watcher`: Uses portable listing; interval tuning by path kind.
- `internal/config`: History uses canonical `smb://`.
- `internal/errors`: Network/auth error types to be added.

## Future Work
- Share enumeration and network discovery UI.
- Copy/move over SMB with progress and cancellation.
- Windows long-path handling policy audit (`\\?\UNC\...`).
