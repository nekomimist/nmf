# UNC Support Plan (Windows & Linux)

This document outlines how to add UNC path support to nmf across Windows and Linux with a consistent UX, minimal churn to existing UI code, and clear integration points.

## Goals
- Windows: Handle `\\\\server\\share\\path` and `\\\\?\\UNC\\server\\share\\path` natively via standard library calls.
- Linux: Treat `smb://server/share/path` (and `//server/share/path`) as remote SMB. Prefer existing mounts if available; otherwise access via SMB client.
- Keep UI, history, watcher, and icon pipelines unchanged from the caller’s perspective by introducing a small VFS abstraction.

## Non-Goals (Initial Phase)
- Full network browsing (NetBIOS/WS-Discovery) and share enumeration.
- Optimistic write operations or file copy/move over SMB (read-only/metadata + directory listing is sufficient initially).
- Inotify/Fanotify on SMB mounts; continue polling for remote.

## Architecture: VFS Abstraction
Introduce a minimal virtual filesystem interface used by code paths that list directories and show file metadata.

- Interface (lives under `internal/fileinfo` to co-locate with metadata types):
  - `type FS interface {`  
    `  ReadDir(ctx context.Context, path string) ([]*Info, error)`  
    `  Stat(ctx context.Context, path string) (*Info, error)`  
    `  Open(ctx context.Context, path string) (io.ReadCloser, error) // optional for preview`  
    `  Join(elem ...string) string`  
    `  Base(path string) string`  
    `  Capabilities() Capabilities`  
    `}`
  - `type Capabilities struct { FastList bool; Watch bool }`
  - Reuse/return existing `internal/fileinfo.Info` or an adapter that nmf already understands.

- Resolver:
  - `Resolve(raw string) (fs FS, parsed Parsed, norm string, err error)`
  - `Parsed`: normalized representation used by history and display: `Scheme ("file" | "smb")`, `Host`, `Share`, `Segments []string`, `Raw string`, `Display string`.
  - Responsibility: detect path flavor, normalize to a display-stable URL (`smb://`), map to an FS provider and a provider-native path.

## Providers
- LocalFS (all OS):
  - Backed by standard `os`/`io/fs` for local paths. On Windows, LocalFS also handles UNC as-is.
  - On Linux, LocalFS handles truly local paths and already-mounted SMB/CIFS paths.
  - Capabilities: `{ FastList: true, Watch: true (local), false (when path is known network mount) }`.

- SMBFS (primarily Linux):
  - Backed by an SMB2 client library (e.g., `github.com/hirochachacha/go-smb2`) to implement `ReadDir/Stat/Open`.
  - Uses credentials from a `CredentialsProvider` (see Auth).
  - Capabilities: `{ FastList: false, Watch: false }`.

## Resolver Rules
- Windows:
  - If input is a UNC path `\\\\server\\share\\...`, use LocalFS directly (optionally long-path prefix `\\\\?\\UNC\\...` for calls that need it).
  - If input is `smb://server/share/...`, convert to `\\\\server\\share\\...` internally and use LocalFS.

- Linux:
  - If input is `smb://server/share/...` (or `//server/share/...`), prefer an existing mount:
    - Inspect `/proc/self/mountinfo` (or `/proc/mounts`) for CIFS/SMB entries that match `server/share` and map to a local mountpoint.
    - If found, rewrite to that local mount path and use LocalFS.
    - If not found, use SMBFS with credentials.
  - Guard against mis-parsing `//server/share` as `/server/share` by explicit detection in the resolver.

## Path Model & Normalization
- Internal vs Display:
  - Display string is always a canonical URL form: `smb://server/share/path`. This stabilizes navigation history across OSes.
  - Provider-native strings:
    - Windows LocalFS: `\\\\server\\share\\path` (with optional `\\\\?\\UNC\\` prefix for long paths when needed).
    - Linux LocalFS: local absolute path (mountpoint + segments).
    - Linux SMBFS: keep parsed host/share/segments and let the provider convert.
- Avoid `filepath` for cross-OS join/base in resolver; use `FS.Join/Base`.

## Credentials & Auth Flow
- `CredentialsProvider` interface:
  - `Get(ctx, host, share) (domain string, user string, password []byte, persist bool, err error)`
- Sources and priority:
  - URL-embedded creds `smb://user[:pass]@host/share` (use once; never log).
  - Config-managed credentials in `internal/config` (opt-in persistence per host/share).
  - UI prompt dialog (new lightweight dialog under `internal/ui`).
- Storage:
  - Start with app config. Consider OS keyring integration as a later enhancement.

## Directory Watching
- Adapt `internal/watcher.DirectoryWatcher` to accept a `ListFunc` backed by `FS.ReadDir`.
- Tuning based on `FS.Capabilities`:
  - Fast local: existing cadence.
  - SMB/remote: longer poll interval (e.g., 2–5s), timeouts, and gentle error handling on network blips.
- Ensure contexts for cancellation and shutdown to avoid leaks.

## UI/UX Notes
- Path input accepts bare local paths, UNC (Windows), and `smb://` URLs (cross-OS). Normalize to `smb://` for display/history.
- Error messages distinguish: auth required/failed, host unreachable, share not found, timeout.
- Navigation history and cursor memory should use the canonical display string (`smb://...`).
- Icons:
  - Windows: UNC uses existing `IconService` behavior.
  - Linux: `icon_unix.go` continues to use extension/MIME; keep batched refresh to hide latency.

## Error Types & Retry
- Add network-aware error wrappers in `internal/errors`:
  - `ErrAuthRequired`, `ErrAuthFailed`, `ErrHostUnreachable`, `ErrShareNotFound`, `ErrTimeout`.
- Conservative retries: exponential backoff 1–3 attempts for transient network errors. Prompt user for explicit retry on auth.

## Testing Strategy
- Resolver & normalization (table-driven):
  - Windows UNC ⇔ `smb://` round-trip, edge cases (trailing slash, `..`, empty segments, long paths).
- FS mocking:
  - Mock `FS` to test watcher diffing and list flows without network.
- Platform-specific tests via build tags for Windows/Linux parsing rules.
- Skip live SMB in CI; provide a short manual verification checklist.

## Rollout Roadmap
1) Introduce `FS`/`Capabilities`/`Resolver`/`Parsed`; refactor list/metadata call sites to use `FS`.
2) Windows validation using LocalFS only (UNC end-to-end).
3) Linux mount detection (`/proc/self/mountinfo`) and LocalFS mapping.
4) Add SMBFS provider and `CredentialsProvider` + login dialog (Linux first).
5) Make `DirectoryWatcher` VFS-aware; adjust intervals/timeouts for SMB.
6) Normalize history/config to `smb://` and add unit tests.

## Touch Points (Likely Files)
- `internal/fileinfo`: new `fs.go` (interfaces), `resolver.go`, and provider adapters.
- `internal/ui`: optional login dialog; path entry normalization.
- `internal/watcher`: make list source pluggable; tune intervals.
- `internal/config`: credentials storage model (opt-in), history normalization.
- `internal/errors`: new network/auth error types.

## Future Work
- OS keyring integration for credential storage.
- Share enumeration and network discovery UI.
- Copy/move over SMB with progress and robust cancellation.
- Windows long path handling audit with `\\\\?\\UNC\\` prefix on edge cases.

