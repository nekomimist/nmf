# NMF Architecture Overview

## Scope

This document describes runtime composition, package boundaries, and core state ownership.

## Runtime Startup Flow

1. `main.go`
   - Parse CLI flags (`-d`, `-path`) and normalize startup path via `resolveDirectoryPath` (`internal/fileinfo.ResolveDirectoryPath`).
   - Load config via `internal/config.Manager`.
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
   - `navigation_ui.go`: navigation dialogs and path entry operations.
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

## Package Boundaries

- `internal/config`: configuration schema and async persistence.
- `internal/fileinfo`: path resolver, VFS abstraction, platform file openers, SMB support, icon service.
- `internal/watcher`: polling watcher with run-generation lifecycle protection.
- `internal/jobs`: copy/move queue manager and background worker.
- `internal/keymanager`: stacked key handlers and modifier state.
- `internal/ui`: dialogs, wrappers, and visual widgets.

## Configuration Model

Source of truth: `internal/config/config.go`.

User-facing `config.json` syntax and examples are documented in
`docs/configuration.md`.

Top-level config sections:

- `window`: `width`, `height`
- `theme`: `dark`, `fontSize`, `fontName`, `fontPath`
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
`S-J`, or `F2` to stable internal command IDs. `externalCommands` define the
commands shown from the main-screen external command menu.

Operational notes:

- Use `Manager.SaveAsync` for interactive updates.
- Call `Manager.Close` on shutdown to flush pending writes.

## Architecture Invariants

- `main.go` remains process-entry focused; runtime orchestration belongs to `FileManager` and split modules.
- New cross-cutting behavior must be documented under `docs/architecture/` in the same change.
