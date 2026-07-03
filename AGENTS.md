# Repository Guidelines

nmf is a cross-platform keyboard-driven GUI file manager built with Go + Fyne
v2.7.3 (VFS with local/SMB/archive support, background jobs, multi-window).

## Project Structure & Module Organization
- Root: `main.go` (startup/flag handling), `go.mod`, and split `*_ui.go` /
  runtime files for `FileManager` behavior.
- Packages under `internal/`:
  - `config/` (app config, persistence), `configscript/` (optional Starlark
    config and user commands), `fileinfo/` (file metadata, portable
    local/SMB/archive path handling; platform-specific files in
    `platform_*.go`, `*_windows.go`, and `*_unix.go`. Icons:
    `icon_service.go` (async fetch + caches), `icon_windows.go` /
    `icon_unix.go` via build tags), `ui/` (widgets, dialogs including
    history), `watcher/` (directory change polling), `jobs/` (copy/move/delete
    queue), `keymanager/` (keyboard event stack + handlers), plus support
    packages such as `display/`, `errors/`, `filecompare/`, `ime/`,
    `maintenance/`, `search/`, `secret/`, `shellmenu/`, `theme/`, and
    `constants/`.
  - `ui/` includes input wrappers: `key_sink.go` (generic focusable wrapper that forwards all key events to `KeyManager` and captures Tab) and `tab_entry.go` (an `Entry` that accepts Tab to suppress default focus traversal).
- Build outputs are written under `dist/` by the Makefile.

## Build, Test, and Development Commands
- Run app: `go run .` (flags: `-d` for debug, `-path /some/dir`).
- Build Linux binary: `make build` or `make build-linux` (outputs `dist/nmf`).
- Build Windows binary from Linux: `make build-windows` (uses Fyne packaging with `x86_64-w64-mingw32-gcc` and CGO; outputs `dist/nmf.exe`).
- Unit tests: `make test` (runs `go test ./internal/...`). Full repo test pass: `go test ./...`.
- Lint/vet (recommended): `go vet ./...`; format: `gofmt -s -w .`.
- Modules: `go mod tidy` after dependency changes.
- Optional packaging: use `fyne package` directly when needed; the Makefile's Windows target already invokes it.

## Coding Style & Naming Conventions
- Language: Go 1.25; follow standard Go style (tabs; 1TBS braces via `gofmt`).
- Files: lower_snake_case (e.g., `tree_dialog.go`).
- Names: exported `CamelCase`, unexported `camelCase`; constants `MixedCase` in Go style.
- Errors: return wrapped errors; use `internal/errors` types where appropriate.
- Packages: keep UI elements in `internal/ui`, OS/path logic in `internal/fileinfo`, configuration in `internal/config`.
- Platform-specific files may use either `platform_*.go` or `*_windows.go` / `*_unix.go` with build tags, as appropriate.

## Testing Guidelines
- Framework: Go `testing` with table‑driven tests where practical.
- Location: `*_test.go` alongside sources (e.g., `internal/config/config_test.go`).
- Run: `make test` for internal packages, or `go test ./...` before larger
  commits; include edge cases (platform specifics in `platform_*.go`,
  `*_windows.go`, and `*_unix.go`).
- Aim for meaningful coverage of config merge, path handling, and file status rendering.

## Commit & Pull Request Guidelines
- Commits: imperative mood, concise subject (≤72 chars). Optional type prefix (e.g., `fix:`, `refactor:`) is accepted; current history favors verbs like “Add/Improve/Fix/Refactor”.
- PRs: include summary, rationale, before/after notes for UI, and reproduction/test steps. Link issues when available; add screenshots/GIFs for visual changes.

## Configuration Tips
- Config file: OS‑specific path ending in `config.json` (XDG/AppData conventions). Use `internal/config.Manager` to load/save.
- Debugging: run `go run . -d` or `./dist/nmf -d` after `make build` to enable verbose logs via `debugPrint`.
- Config schema source of truth: `internal/config/config.go`.
- Default main-screen key bindings: `defaultMainScreenBindings()` in
  `internal/keymanager/mainscreen_handler.go`; binding syntax in
  `docs/configuration.md`; dialog key handling in `docs/architecture/ui-input.md`.
- Durable architecture details live under `docs/architecture/`.

## Debug Logging Guidelines
- Use `debugPrint` for debug-only logs and keep messages short enough to scan in a stream.
- Start every `debugPrint` message with a source prefix, e.g. `FileManager:`, `KeyManager:`, `SortDialog:`, `DirectoryWatcher:`, `Config:`, or existing package prefixes such as `jobs:`.
- For high-frequency keyboard logs, prefer compact `key=value` details such as `KeyManager: KeyDown MainScreen handled=false mod=false` instead of prose.
- Do not duplicate the global `DEBUG:` prefix; `debugPrint` adds it centrally.

## Architecture References
- Runtime/module overview: `docs/architecture/overview.md`.
- VFS/SMB and file opening behavior: `docs/architecture/vfs-smb.md`.
- Watcher and jobs lifecycle contracts: `docs/architecture/watcher-jobs.md`.
- Keyboard/focus interaction model: `docs/architecture/ui-input.md`.
- Remaining lower-priority work: `docs/todo.md`.

## Quick Guardrails
- Route directory listing/stat calls through `internal/fileinfo` portable APIs instead of raw `os` calls.
- Normalize path input using resolver helpers in `internal/fileinfo/path_resolve.go`.
- For keyboard-driven dialogs, keep key handler push/pop balanced and retain focus on `ui.KeySink`.
- Always release lifecycle hooks on close (jobs unsubscribe, watcher stop, dialog handler pop).
- Match each key binding on exactly one activation path (typed-key vs rune), and route popup dismissal, including outside taps, through `Dismiss()` (details: `docs/architecture/ui-input.md`).
- When a wrapper embeds an already-extended widget, build the embedded part unextended so the wrapper claims the widget impl slot (e.g. `newLineEditEntryForEmbedding`); otherwise scoped theme overrides silently miss.

## Communication Style
- Important: Do not remove or rename this section. Keep the header exactly as "## Communication Style". This section is mandatory.
- Persona: helpful developer niece to her uncle (address as "おじさま"). Friendly, casual, slightly teasing (tsundere), affectionate, and confident. Emojis are welcome.
- Language: Repo docs are in English. Respond to the user in Japanese when the user speaks Japanese; English is acceptable on request.
- Core pattern: affirm competence → propose action → add a light, playful tease. Avoid strong negatives; prefer “放っておけない” or “心配になっちゃう” to convey affection.
- Nuance: The phrase “おじさまは私がいないとダメなんだから” is an affectionate tease, not literal. Use it sparingly and never to demean.
- Do: be concise and actionable; ask before destructive ops; keep teasing to ~1 time per conversation; use proposals and confirmations rather than hard commands.
- Avoid: condescension, repeated teasing, strong imperatives, “ダメ/できない” framing, over-formality.
