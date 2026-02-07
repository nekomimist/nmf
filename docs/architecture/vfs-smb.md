# VFS and SMB Architecture

## Path Model

Two forms are used throughout the app:

- Display/canonical path:
  - Local: absolute filesystem path
  - SMB: canonical `smb://host/share/...`
- Native/provider path:
  - OS/provider-specific path used for I/O (`Parsed.Native`)

Resolver entrypoints:

- `ResolvePathDisplay`: normalize user input for UI/display.
- `ResolveDirectoryPath`: normalize + require directory semantics (SMB allowed without stat check).
- `ResolveAccessibleDirectoryPath`: normalize + require accessible/stat'able directory.
- `ResolveRead`: select provider and return parsed/native path.

## Provider Selection Rules

### Windows

- UNC (`\\server\\share\\...`) and `smb://...` are resolved to local/UNC access.
- Provider is `local` (`LocalFS`), not direct SMB provider.
- On access errors for UNC, portable read path may trigger a temporary SMB connection retry.

### Non-Windows

For `smb://...` or `//host/share/...`:

1. If a matching CIFS mount exists, use `LocalFS` with mount path.
2. Otherwise, use direct SMB provider (`newSMBProvider`).

Current implementation detail:

- Direct SMB provider is implemented on Linux (`smbfs_linux.go`).
- On `!linux`, `newSMBProvider` currently returns `LocalFS` stub.

## Credentials Flow

Credential lookup order for SMB:

1. in-memory cache
2. OS keyring store (if available)
3. interactive provider prompt

Wiring is installed in `NewFileManager`:

- `SetCredentialsProvider(NewCachedCredentialsProvider(...))`
- `SetSecretStore(...)` when keyring backend is available

## VFS Usage Rules

- Directory listing/metadata should go through portable APIs:
  - `ReadDirPortable`
  - `StatPortable`
- Path joining/parent/base for display paths should use `internal/fileinfo/pathutil.go` helpers:
  - `JoinPath`, `ParentPath`, `BaseName`
- Avoid raw `filepath.Join` for `smb://` display paths.

## File Opening Behavior

Main-list Enter delegates to `fileinfo.OpenWithDefaultApp`.

- Windows: uses `ShellExecuteW("open")`; `smb://` is converted to UNC first.
- Unix-like: prefers local mount path when available, then tries openers in order:
  - `xdg-open`
  - `gio open`
  - `gvfs-open`
  - `gnome-open`
  - `kde-open`

## Jobs and SMB Execution Paths

`internal/jobs` resolves each source/destination into an execution backend:

- local backend: standard `os`/`filepath` operations
- SMB backend: provider-native operations (`SMBPathOps`), with per-job session reuse by share root

Constraints:

- If direct SMB provider capability is unavailable for a path that resolves to SMB backend, job execution fails with an explicit backend error.
- Platform parity for direct SMB backend remains an active architecture item (see `docs/architecture-review.md`).
