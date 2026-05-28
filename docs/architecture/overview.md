# NMF Architecture Overview

## Scope

This document describes runtime composition, package boundaries, and core state ownership.

## Runtime Startup Flow

1. `main.go`
   - Parse CLI flags (`-d`, `-path`) and normalize startup path via `resolveDirectoryPath` (`internal/fileinfo.ResolveDirectoryPath`).
   - Load config via `internal/config.Manager`, then apply optional
     `init.star` via `internal/configscript`.
   - Create Fyne app and apply custom theme.
   - Install jobs debug hook (`internal/jobs.SetDebug`).
2. `bootstrap.go` (`NewFileManager`)
   - Construct `FileManager` state.
   - Initialize icon service, directory watcher, SMB credential providers, and key manager.
   - Build UI (`setupUI`), load initial directory (`LoadDirectory`), then start watcher.
   - Register window close intercept (`closeWindow`).
3. Runtime method groups are split across focused files:
   - `directory_loading.go`: loading, busy state, watcher poll policy.
   - `list_controls.go`: sorting/filter/search/list cursor operations.
   - `navigation_ui.go`: navigation dialogs and path edit operations.
   - `viewer_ui.go`: built-in text/Markdown/hex preview dialog entrypoint.
   - `jobs_ui.go`: job enqueue/indicator integration.
   - `window_lifecycle.go`: close/quit cleanup logic.

## Core State Ownership

`FileManager` (`file_manager.go`) owns UI/runtime state for a single window:

- current path + file list snapshot
- selection and cursor state
- key manager stack and search overlay
- watcher instance and jobs indicator state
- config/config manager handles

Cross-window/global state:

- window registry and count in `main.go`
- singleton jobs manager in `internal/jobs`

Window placement:

- `Ctrl-N` creates a new `FileManager` beside the source window when the
  platform supports native placement.
- Windows uses `driver.NativeWindow` plus Win32 `HWND` APIs in
  `window_position_windows.go`.
- Other platforms intentionally use the window manager's default placement via
  `window_position_other.go`.

Other platform-specific desktop integrations, including native shell context
menus and outbound file dragging, are summarized in `platform-behavior.md`.

## Package Boundaries

- `internal/config`: configuration schema and async persistence.
- `internal/configscript`: optional Starlark overlay configuration and custom command registration.
- `internal/fileinfo`: path resolver, VFS abstraction, platform file openers,
  bounded preview loading, SMB support, icon service.
- `internal/watcher`: polling watcher with run-generation lifecycle protection.
- `internal/jobs`: copy/move queue manager and background worker.
- `internal/keymanager`: stacked key handlers and modifier state.
- `internal/ui`: dialogs, wrappers, and visual widgets.

## Configuration Model

Source of truth: `internal/config/config.go`.

User-facing `config.json` syntax and examples are documented in
`docs/configuration.md`. Optional Starlark configuration is documented in
`docs/starlark-configuration.md`.

Top-level config sections:

- `window`: `width`, `height`
- `theme`: `dark`, `fontSize`, `fontName`, `fontPath`, `colors`
- `ui`:
  - `showHiddenFiles`, `sort`, `itemSpacing`
  - `cursorStyle`
  - `cursorMemory`
  - `navigationHistory`
  - `fileFilter`
  - `directoryJumps`
  - `keyBindings`
  - `externalCommands`

Main-screen keyboard shortcuts are resolved through the key manager command
registry. Configured `keyBindings` map key specifications such as `C-N`,
`S-J`, `S-Q`, or `F2` to stable internal command IDs. `externalCommands` define the
commands shown from the main-screen external command menu. Runtime-state
maintenance tools are exposed through the `maintenance.show` command.

If `init.star` is present next to `config.json`, it is loaded after JSON and
before Fyne theme/window construction. Starlark can overlay all user-editable
configuration fields, append or replace list-style configuration, and register
`user.*` command IDs for key bindings. Config saves pass through a transform
that restores Starlark-owned fields to the pre-overlay JSON values while
preserving runtime state.

Operational notes:

- Use `Manager.SaveAsync` for interactive updates.
- Call `Manager.Close` on shutdown to flush pending writes.
- Navigation history keeps `lastUsed` and `useCount` runtime stats, sorts saved
  entries by zoxide-style frecency, and defaults to retaining 10000 paths.

## Architecture Invariants

- `main.go` remains process-entry focused; runtime orchestration belongs to `FileManager` and split modules.
- New cross-cutting behavior must be documented under `docs/architecture/` in the same change.
