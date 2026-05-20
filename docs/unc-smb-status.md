# UNC/SMB Support Status (Windows & Linux)

This document tracks UNC/SMB support across Windows and Linux. UI callers should continue to use canonical display paths and portable file APIs instead of provider-specific path handling.

## Status Snapshot

- Completed:
  - Resolver and normalization: Windows UNC <-> `smb://` conversion; Linux `smb://` and `//host/share` mount detection via `/proc/self/mountinfo`; fallback to direct SMB on Linux.
  - VFS surface includes `ReadDir`, `Stat`, `Open`, `Join`, `Base`, and provider `Capabilities`.
  - Portable APIs: `ReadDirPortable`, `StatPortable`, and display path helpers (`JoinPath`, `ParentPath`, `BaseName`).
  - Windows UNC listing retry through a temporary `WNetAddConnection2W` session on access errors.
  - Linux direct SMB provider using `github.com/hirochachacha/go-smb2`, including listing, metadata, open, write, mkdir, remove, rename, and symlink operations.
  - SMB credentials UI, in-memory cache, OS keyring integration, and lookup order of memory -> keyring -> UI.
  - URL credentials are parsed and can seed the in-memory cache during path-entry navigation.
  - Path input normalization; SMB navigation/history uses canonical `smb://` display paths.
  - Directory tree dialog uses portable VFS (`ReadDirPortable`/`StatPortable`).
  - Directory watcher uses portable listing and a longer poll interval for `smb://` paths.
  - Copy/move jobs support Linux direct SMB paths through `SMBPathOps`, with per-job session reuse per share root.
  - Unit tests cover resolver helpers, UNC conversion helpers, mountinfo parsing helpers, path utilities, credentials precedence, watcher lifecycle/change detection, and SMB job backend/session behavior.

- Remaining:
  - Network/auth error typing, friendly UI messages, and conservative retry/backoff for transient SMB failures.
  - Watcher list source injection and capability-based tuning instead of `smb://` string heuristics.
  - Non-Linux direct SMB provider/copy-move behavior policy and tests.
  - SMB integration tests are manual only; CI fixture coverage is not wired.
  - Windows long-path (`\\?\UNC\...`) policy and edge-case audit.
  - Credential cache lifecycle details for multiple windows and startup URL credentials.

## Goals

- Windows: Handle `\\server\share\path` and `smb://server/share/path` through native UNC access, with credential retry on access failures.
- Linux: Treat `smb://server/share/path` and `//server/share/path` as SMB; prefer existing CIFS/SMB mounts, otherwise use the direct SMB provider.
- Keep UI, history, watcher, tree dialog, file opening, and jobs code stable through the resolver/VFS boundary.

## Non-Goals

- Network browsing and share enumeration.
- Kernel filesystem notifications for SMB; remote paths continue to use polling.
- Full direct-SMB provider parity on non-Linux until the platform support policy is decided.

## Architecture: VFS and Resolver

- Implemented VFS:
  - `ReadDir(path string) ([]os.DirEntry, error)`
  - `Stat(path string) (os.FileInfo, error)`
  - `Open(path string) (io.ReadCloser, error)`
  - `Join(elem ...string) string`
  - `Base(path string) string`
  - `Capabilities() Capabilities`
- Portable callers should use `ReadDirPortable`, `StatPortable`, and path helpers in `internal/fileinfo/pathutil.go`.
- Windows provider selection:
  - UNC and `smb://` resolve to `LocalFS` over UNC.
  - `ReadDirPortable` retries access errors after attempting a temporary Windows network connection.
- Non-Windows provider selection:
  - Matching CIFS/SMB mount: use `LocalFS` with the mount path.
  - No mount on Linux: use `SMBFS`.
  - No mount on `!linux`: direct SMB provider is unavailable; this remains an active architecture item.

## Current Active Work Items

- Error handling:
  - Add network-aware error wrappers under `internal/errors` for auth required/failed, host unreachable, share not found, timeout, and credential conflict.
  - Map those errors to friendly UI dialog messages instead of showing raw provider errors.
  - Add bounded retry/backoff only for transient network failures; auth failures should clear stale cached credentials and prompt again.
- Watcher:
  - Add injectable list source or provider handle so watcher tests do not depend on live filesystem/SMB state.
  - Use `Capabilities()` to choose polling behavior instead of `strings.HasPrefix(path, "smb://")`.
- Platform parity:
  - Decide whether non-Linux direct `smb://` copy/move should be unsupported-by-design, Windows-native-UNC backed, or implemented through a provider.
  - Document and test the selected behavior.
- Test infrastructure:
  - Add repeatable SMB integration coverage, preferably with a dockerized Samba fixture or a gated CI job.
  - Keep the existing `NMF_SMB_TEST_DIR=smb://host/share/path go test ./internal/jobs -run TestSMBCopyRoundtrip` path for manual validation.
- Credentials:
  - Preserve memory cache across multiple windows or make the reset behavior explicit.
  - Ensure startup paths such as `-path smb://user:pass@host/share` seed URL credentials before first SMB access.
- Windows path policy:
  - Audit `\\?\UNC\...` long-path behavior for resolver, display normalization, file opening, and Windows connection retry.

## Touch Points

- `internal/fileinfo`: VFS, resolver, portable APIs, path helpers, SMB providers, credentials, Windows UNC connection helper.
- `internal/jobs`: copy/move execution backends, `SMBPathOps`, SMB session reuse.
- `internal/watcher`: portable listing, polling lifecycle, pending capability-based tuning.
- `internal/ui`: SMB login dialog, tree dialog, path/history dialogs, user-facing error presentation.
- `docs/architecture-review.md`: active unresolved architecture and reliability risks.

## Future Work

- Share enumeration and network discovery UI.
- Full cross-platform direct SMB provider parity if product scope requires it.
- Richer copy/move conflict handling over SMB, including overwrite policy and partial artifact cleanup.
