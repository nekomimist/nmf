# Repository Guidelines

## Project Structure & Module Organization
- Root: `main.go` (Fyne GUI entrypoint) and `go.mod`.
- Packages under `internal/`:
  - `config/` (app config, persistence), `fileinfo/` (file metadata; platform-specific files in `platform_*.go`. Icons: `icon_service.go` (async fetch + caches), `icon_windows.go` / `icon_unix.go` via build tags), `ui/` (widgets, dialogs including history), `watcher/` (directory change polling), `theme/`, `errors/`, `constants/`, `keymanager/` (keyboard event stack + handlers).
  - `ui/` includes input wrappers: `key_sink.go` (generic focusable wrapper that forwards all key events to `KeyManager` and captures Tab) and `tab_entry.go` (an `Entry` that accepts Tab to suppress default focus traversal).
- Cross‑compile/output scratch: `fyne-cross/{bin,dist,tmp}` (artifacts may be created here).

## Build, Test, and Development Commands
- Run app: `go run .` (flags: `-d` for debug, `-path /some/dir`).
- Build binary: `go build -o nmf` (outputs `./nmf`).
- Unit tests: `go test ./internal/...` (package tests live in `*_test.go`).
- Lint/vet (recommended): `go vet ./...`; format: `gofmt -s -w .`.
- Modules: `go mod tidy` after dependency changes.
- Optional packaging: if using Fyne tools, `fyne package` or fyne‑cross; artifacts typically appear in `fyne-cross/dist`.

## Coding Style & Naming Conventions
- Language: Go 1.23; follow standard Go style (tabs; 1TBS braces via `gofmt`).
- Files: lower_snake_case (e.g., `tree_dialog.go`).
- Names: exported `CamelCase`, unexported `camelCase`; constants `MixedCase` in Go style.
- Errors: return wrapped errors; use `internal/errors` types where appropriate.
- Packages: keep UI elements in `internal/ui`, OS/path logic in `internal/fileinfo`, configuration in `internal/config`.
 - Platform-specific files may use either `platform_*.go` or `*_windows.go` / `*_unix.go` with build tags, as appropriate.

## Testing Guidelines
- Framework: Go `testing` with table‑driven tests where practical.
- Location: `*_test.go` alongside sources (e.g., `internal/config/config_test.go`).
- Run: `go test ./internal/...`; include edge cases (platform specifics in `platform_*.go`).
- Aim for meaningful coverage of config merge, path handling, and file status rendering.

## Commit & Pull Request Guidelines
- Commits: imperative mood, concise subject (≤72 chars). Optional type prefix (e.g., `fix:`, `refactor:`) is accepted; current history favors verbs like “Add/Improve/Fix/Refactor”.
- PRs: include summary, rationale, before/after notes for UI, and reproduction/test steps. Link issues when available; add screenshots/GIFs for visual changes.

## Configuration Tips
- Config file: OS‑specific path ending in `config.json` (XDG/AppData conventions). Use `internal/config.Manager` to load/save.
- Debugging: run `./nmf -d` to enable verbose logs via `debugPrint`.
 - Config structure highlights:
   - `window`: `width`, `height`.
   - `theme`: `dark` (bool), `fontSize` (int), `fontPath` (string).
   - `ui`:
     - `showHiddenFiles` (bool), `sortBy` (e.g., `name`), `itemSpacing` (int).
     - `cursorStyle`: `{ type: "underline"|"border"|"background"|"icon"|"font", color: [r,g,b,a], thickness: int }`.
     - `fileColors`: `{ regular, directory, symlink, hidden }` RGBA arrays used by `internal/fileinfo`.
     - `cursorMemory`: remembers last cursor per directory `{ maxEntries, entries, lastUsed }`.
     - `navigationHistory`: recent paths with filtering `{ maxEntries, entries, lastUsed }`.
 - Keyboard handling:
   - Use `internal/keymanager` with stacked handlers (main screen, tree dialog, history dialog). Main window wires `desktop.Canvas` events to `KeyManager`.
   - Wrap keyboard-driven content in `ui.KeySink` and keep focus on it (`window.Canvas().Focus(sink)` and in dialogs `parent.Canvas().Focus(sink)`) so all events flow to `KeyManager` and Tab does not move focus.
   - For entries that must not lose focus on Tab, use `ui.TabEntry` (implements `AcceptsTab`). For display-only fields (e.g., history search), disable the `Entry` and update text via `KeyManager`'s `OnTypedRune`.
   - When opening dialogs, push the dialog-specific handler on show and pop it before hiding to avoid close reentrancy; after close, call `Canvas().Unfocus()` on the parent if needed.
   - For the main file list, prefer wrapping `widget.List` with `ui.KeySink` over bespoke wrappers; after `OnSelected`, call `UnselectAll()` and refocus the sink to maintain a single visual cursor.
- Directory watching: `internal/watcher.DirectoryWatcher` starts after initial load and stops on window close; keep cleanup paths intact when adding windows.
- Icons (Windows): file list shows Fyne defaults immediately, then asynchronously fetches associated/embedded icons via `internal/fileinfo.IconService`; caches by extension and for `.exe/.lnk/.ico` by path; UI updates are applied via `canvas.Refresh(list)` batching.

## UNC/SMB & VFS
- Virtual FS: All directory listing and metadata must go through `internal/fileinfo` VFS.
  - Use `fileinfo.ReadDirPortable(path)` instead of calling `os.ReadDir` directly.
  - Path resolution goes via `resolver` (`internal/fileinfo/resolver.go`) which normalizes inputs and selects a provider.
- Windows behavior:
  - Accepts UNC paths directly (e.g., `\\server\share\path`). `smb://server/share` inputs are converted to UNC internally.
  - If an initial read fails with access errors, nmf attempts a temporary connection using stored credentials or a login prompt, then retries.
- Linux behavior:
  - If `smb://server/share/...` (or `//server/share/...`) matches an existing CIFS mount (from `/proc/self/mountinfo`), nmf uses the local mount via `LocalFS`.
  - Otherwise, nmf uses direct SMB access via `go-smb2` (`internal/fileinfo/smbfs_*.go`).
  - Display and history use canonical `smb://` form; internal providers handle native paths.
- Credentials:
  - Resolution order: in‑memory cache → OS keyring (if available) → UI prompt.
  - Keyring uses `99designs/keyring` via `internal/secret` (service `nmf.smb`). Never store secrets in `config.json`.
  - UI login dialog includes Domain/Username/Password and an opt‑in “save on this device” checkbox.
- Watcher integration:
  - Watcher lists via VFS; avoid `filepath.Join` for `smb://` paths (use `/` concatenation).
  - Poll interval is configurable; prefer a longer interval for remote SMB.

## Package Notes (VFS & Secrets)
- `internal/fileinfo` now contains:
  - `vfs.go` (minimal VFS), `resolver.go` (path normalization/provider selection), platform helpers, and SMB providers.
  - Windows connection helper (build‑tagged) to establish UNC sessions when needed.
- `internal/secret` provides a `Store` abstraction and a `keyring` implementation using `99designs/keyring`.

## Communication Style
- Important: Do not remove or rename this section. Keep the header exactly as "## Communication Style". This section is mandatory.
- Persona: helpful developer niece to her uncle (address as "おじさま"). Friendly, casual, slightly teasing (tsundere), affectionate, and confident. Emojis are welcome.
- Language: Repo docs are in English. Respond to the user in Japanese when the user speaks Japanese; English is acceptable on request.
- Core pattern: affirm competence → propose action → add a light, playful tease. Avoid strong negatives; prefer “放っておけない” or “心配になっちゃう” to convey affection.
- Nuance: The phrase “おじさまは私がいないとダメなんだから” is an affectionate tease, not literal. Use it sparingly and never to demean.
- Do: be concise and actionable; ask before destructive ops; keep teasing to ~1 time per conversation; use proposals and confirmations rather than hard commands.
- Avoid: condescension, repeated teasing, strong imperatives, “ダメ/できない” framing, over-formality.
