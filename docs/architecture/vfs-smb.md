# VFS and SMB Architecture

## Goals and Non-Goals

Goals:

- Windows: handle `\\server\share\path` and `smb://server/share/path`
  through native UNC access, with credential retry on access failures.
- Linux: treat `smb://server/share/path` and `//server/share/path` as SMB;
  prefer existing CIFS/SMB mounts, otherwise use the direct SMB provider.
- Keep UI, history, watcher, tree dialog, file opening, and jobs code stable
  through the resolver/VFS boundary.

Non-goals:

- Network browsing and share enumeration.
- Kernel filesystem notifications for SMB; remote paths use polling.
- Full direct-SMB provider parity on non-Linux until the platform support
  policy is decided.

## Path Model

Two forms are used throughout the app:

- Display/canonical path:
  - Local: absolute filesystem path
  - SMB: canonical `smb://host/share/...`; host names are lower-cased, and
    WSL's `wsl$` host alias is normalized to `wsl.localhost`
  - Archive: `outer.zip!/inner/path`
- Native/provider path:
  - OS/provider-specific path used for I/O (`Parsed.Native`)
  - Archive: path inside the archive, rooted at `.`

Resolver entrypoints:

- `ResolvePathDisplay`: normalize user input for UI/display.
- `CanonicalDisplayPath`: normalize user input for display/history without
  requiring accessibility.
- `ResolveDirectoryPath`: normalize + require directory semantics (SMB allowed without stat check).
- `ResolveAccessibleDirectoryPath`: normalize + require accessible/stat'able directory.
- `ResolveRead`: select provider and return parsed/native path.

Path input and history use canonical display paths. SMB URL credentials can be
parsed from path-entry navigation and used to seed the in-memory credential
cache before provider access.

Archive paths are read-only. The root display path for an archive is
`archive-file!/`; nested archive navigation is intentionally unsupported.
ZIP entry names without the UTF-8 flag use the configured fallback charset
(`ui.archive.zipNameEncoding`, default `shift_jis`); valid UTF-8 names are kept
as UTF-8 even when the flag is absent.
Password-protected 7z/RAR archives prompt for a password and cache it in memory
for the current session. Password-protected ZIP archives are not supported by
the current archive backend.

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

Implemented VFS providers expose:

- `ReadDir(path string) ([]os.DirEntry, error)`
- `Stat(path string) (os.FileInfo, error)`
- `Open(path string) (io.ReadCloser, error)`
- `Join(elem ...string) string`
- `Base(path string) string`
- `Capabilities() Capabilities`

- Directory listing/metadata should go through portable APIs:
  - `ReadDirPortable`
  - `StatPortable`
- Path joining/parent/base for display paths should use `internal/fileinfo/pathutil.go` helpers:
  - `JoinPath`, `ParentPath`, `BaseName`
- Avoid raw `filepath.Join` for `smb://` display paths.
- Directory tree dialogs should also use the portable read/stat APIs.
- Directory watcher uses shared fswatcher-backed path sources for watchable
  local paths, then portable listing to refresh snapshots after events. Watcher
  registration failures fall back to polling.
- Direct SMB/archive providers currently report `Watch: false`, so main-window
  directory watching is not started for those paths.

## File Opening Behavior

Main-list `Return` uses the `open` command. It enters directories, enters
supported archive files, and delegates other files to
`fileinfo.OpenWithDefaultApp`.
On Windows, `.lnk` shortcut files are resolved before default-app delegation:
shortcuts to directories enter the target directory, and shortcuts to files
enter the target file's parent directory.

Main-list `Shift+Return` uses `open.defaultApp`. It enters directories but
delegates files directly to `fileinfo.OpenWithDefaultApp`, so archive-like
application formats such as `.xlsx` can be launched with the OS-associated app.

- Windows: uses `ShellExecuteW("open")`; `smb://` is converted to UNC first.
- Unix-like: prefers local mount path when available, then tries openers in order:
  - `xdg-open`
  - `gio open`
  - `gvfs-open`
  - `gnome-open`
  - `kde-open`
- Archive entries: extract the selected file to an `nmf-archive-open-*`
  temporary directory, then delegate to the same platform opener.

When `Return` is pressed on a supported archive file, the UI navigates to
`archive-file!/` instead of launching the archive externally.

## Built-in Preview Viewer

Main-list `V` opens the selected file with the internal viewer instead of the
platform default application.

- Reads go through `ResolveRead` and the selected provider's `VFS.Open`, so
  local, mounted/direct SMB, and archive entry previews share the same path
  model.
- The viewer reads only the first 1 MiB and reports truncation in the dialog
  status.
- Text decoding uses `github.com/gogs/chardet` for non-empty preview data,
  converts the detected charset to valid UTF-8, and displays replacement text
  if detection or conversion fails.
- Text and Markdown tabs operate on decoded text; the hex tab operates on the
  original bytes that were read.

## Jobs and SMB Execution Paths

`internal/jobs` resolves each source/destination into an execution backend:

- local backend: standard `os`/`filepath` operations
- SMB backend: provider-native operations (`SMBPathOps`), with per-job session reuse by share root
- archive backend: read-only `ArchiveVFS` source operations

Constraints:

- If direct SMB provider capability is unavailable for a path that resolves to SMB backend, job execution fails with an explicit backend error.
- Platform parity for direct SMB backend remains a low-priority follow-up item (see `docs/todo.md`).
- Archive paths can be copy sources. Archive destinations, move, rename, and
  delete are rejected because archive mutation is out of scope.

Delete behavior:

- Permanent delete uses the same execution backend model as copy/move.
- Trash delete uses OS trash/recycle APIs for local-provider paths.
- Direct SMB trash is unsupported; users must use explicit permanent delete for
  direct SMB paths.
