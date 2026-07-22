# NMF Architecture Overview

## Scope

This document describes runtime composition, package boundaries, and core state ownership.

## Runtime Startup Flow

1. `main.go`
   - Parse CLI flags (`-d`, `-path`) and normalize startup path via `resolveDirectoryPath` (`internal/fileinfo.ResolveDirectoryPath`).
   - Load config via `internal/config.Manager`, then load runtime state via
     `internal/config.StateManager` (migrating legacy `config.json` runtime
     keys into `state.json` on first run), set up configured debug logging,
     then apply optional `init.star` via `internal/configscript`.
   - Create Fyne app and apply custom theme.
   - Install jobs debug hook (`internal/jobs.SetDebug`).
2. `bootstrap.go` (`NewFileManager`)
   - Construct `FileManager` state from the shared `ApplicationRuntime`.
   - Initialize the window-owned icon service, directory watcher, and key manager.
   - Build UI (`setupUI`) and load the initial directory (`LoadDirectory`).
     The watcher starts after a directory load successfully applies.
   - Register the title-bar close intercept through `QuitApplication`, the
     same confirmation path used by the keyboard command.
3. Runtime method groups are split across focused files:
   - `directory_loading.go`: loading, busy state, watcher poll policy.
   - `list_controls.go`: sorting/filter/search/list cursor operations.
   - `navigation_ui.go`: navigation dialogs and path edit operations.
   - `viewer_ui.go`: built-in image/text/Markdown/hex preview dialog entrypoint.
   - `jobs_ui.go`: job enqueue/indicator integration.
   - `window_lifecycle.go`: close/quit cleanup logic.

## Core State Ownership

`FileManager` (`file_manager.go`) owns UI/runtime state for a single window:

- current path + file list snapshot
- selection and cursor state
- key manager stack and search overlay
- watcher instance and jobs indicator state
- shared watcher hub reference
- config/config manager handles, plus state/state manager handles (`state.json`)

Cross-window/global state:

- window registry and count in `main.go`
- `ApplicationRuntime` owns the shared `internal/watcher.WatchHub`, jobs
  manager/controller, credential and archive-password caches, and the
  interactive prompt broker.
- The VFS provider hooks in `internal/fileinfo` are installed once when the
  runtime is created. Opening another window registers a prompt target but
  does not replace the global cache/provider.
- Interactive SMB, archive-password, and job-conflict prompts are serialized.
  The broker selects an active open window when a request actually needs UI;
  queued jobs retain the application broker rather than their source
  `FileManager`.

Window shutdown:

- Closing the last FileManager window always opens the quit confirmation;
  when jobs are pending or running, the dialog requires an explicit
  `Quit Anyway` action.
- Programmatic destruction is idempotent. Before window-owned subscriptions
  and widgets are released, the active directory load is canceled and its
  generation is invalidated so an already queued completion cannot revive the
  watcher or refocus the closed window.
- Closing a window unregisters its prompt target. Later job conflicts are
  routed to another open window, or cancel conservatively when none remains.
- Each registered prompt target owns a cancellation context. Unregistering a
  window cancels an in-flight SMB, archive-password, or conflict prompt and
  releases the application prompt slot without waiting for a queued Fyne close
  callback; shutdown may discard queued UI functions.
- Each window closes its `IconService` before destroying widgets. Close stops
  its workers and batch notifier, drops callbacks, and makes late icon results
  inert; the callback also checks the window generation before refreshing.
- External commands and OS opener processes are started asynchronously, but a
  lightweight waiter goroutine always calls `Wait` so completed children do
  not remain unreaped.

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

- `internal/config`: read-only `config.json` schema/loading (`Manager`) plus
  `state.json` runtime state and its async persistence (`StateManager`).
- `internal/configscript`: optional Starlark overlay configuration and custom command registration.
- `internal/fileinfo`: path resolver, VFS abstraction, platform file openers,
  bounded preview loading, SMB support, icon service.
- `internal/watcher`: shared fswatcher-backed path monitor with polling
  fallback and run-generation lifecycle protection.
- `internal/jobs`: copy/move queue manager and background worker.
- `internal/keymanager`: stacked key handlers and modifier state.
- `internal/ui`: dialogs, wrappers, and visual widgets.

## Configuration Model

Source of truth: `internal/config/config.go` (`config.json`, read-only from
the app) and `internal/config/state.go` (`state.json`, runtime state).

User-facing `config.json` syntax and examples are documented in
`docs/configuration.md`. Optional Starlark configuration is documented in
`docs/starlark-configuration.md`.

Top-level config sections:

- `window`: `width`, `height`
- `theme`: `dark`, `fontSize`, `fontName`, `fontPath`,
  `monospaceFontName`, `monospaceFontPath`, `colors`
- `debug`: `enabled`, `logDirectory`, `maxLogFiles`
- `ui`:
  - `showHiddenFiles`, `sort`, `itemSpacing`
  - `cursorStyle`
  - `cursorMemory` (`maxEntries` only; see Runtime State below)
  - `navigationHistory` (`maxEntries` only; see Runtime State below)
  - `fileFilter` (`maxEntries` only; see Runtime State below)
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
`user.*` command IDs for key bindings. Since `config.json` is never written by
the app, there is no save-time transform to reconcile Starlark-owned fields
with JSON; the overlay only affects the running process.

Configured debug logging creates one `nmf-*.log` file per startup under the
configured log directory and prunes old matching logs. When enabled, the main
toolbar exposes a mouse-accessible KeyManager state dump for diagnosing input
routing failures.

Operational notes:

- `Manager.Load` is the only operation on `config.json`; the app never saves
  to it.
- Interactive updates (cursor memory, navigation history, file filter, sort)
  go through `StateManager.SaveAsync` against `state.json` instead. See
  "Runtime State (state.json)" below.

## Runtime State (state.json)

Frequently-changing runtime state that used to live in `config.json` —
remembered cursor positions, navigation history (including pinned History
Jump paths), file filter history plus the currently applied filter, and the
last-applied sort — is now owned by `internal/config/state.go`'s `State`
struct and persisted separately to `state.json`, managed by `StateManager`.
Full JSON shape and OS-specific paths are documented in
`docs/configuration.md`'s "Runtime State" section.

- `StateManager` mirrors `Manager`'s former debounced background-save worker:
  `SaveAsync` schedules a write coalesced over a 500ms window, `Flush` forces
  a pending write immediately, and `Close` flushes and stops the worker.
  `main.go` calls `stateManager.Close()` on shutdown so no in-flight state is
  lost.
- Writes are atomic: `saveState` marshals to a temp file in the state
  directory, then renames it over `state.json`, so a crash mid-write cannot
  leave a corrupt file behind.
- One-time migration: `StateManager.Load` seeds `state.json` from legacy
  runtime keys in `config.json` only when `state.json` does not yet exist,
  then writes `state.json` so its existence marks the migration as done.
  `config.json` is never modified by this process; deleting `state.json`
  while old runtime keys remain in `config.json` re-runs the migration.
- Sort precedence: `State.EffectiveSort` returns `state.Sort` when a sort has
  been applied via the Sort dialog, otherwise falls back to `config.json`'s
  `ui.sort`. Navigation history keeps `lastUsed` and `useCount` runtime
  stats, sorts saved entries by zoxide-style frecency, and defaults to
  retaining 10000 paths; `navigationHistory.pinned` stores saved History Jump
  paths outside that pruning limit.

## Architecture Invariants

- `main.go` remains process-entry focused; runtime orchestration belongs to `FileManager` and split modules.
- New cross-cutting behavior must be documented under `docs/architecture/` in the same change.
