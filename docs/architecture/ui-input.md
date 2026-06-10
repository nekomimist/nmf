# UI Input and Focus Model

## Keyboard Routing Architecture

Source packages:

- `internal/keymanager`
- `internal/ui/key_sink.go`
- `internal/ui/tab_entry.go`

Core model:

- `KeyManager` maintains a stack of handlers.
- Active handler is the top of stack.
- `DumpState` returns handler stack, modifier state, pressed keys, pending
  transitions, and suppression flags for debug logs.
- The main-screen handler maps key events to stable internal command IDs before
  executing file-manager behavior.
- Events routed by type:
  - `OnKeyDown`
  - `OnKeyUp`
  - `OnTypedKey`
  - `OnTypedRune`

Modifier keys (`Shift`, `Ctrl`, `Alt`) are tracked centrally in `KeyManager` and passed to handlers.

Event delivery paths:

- Fyne's GLFW driver delivers each key event exclusively: the focused object
  receives it when focus exists, and the canvas-level callbacks fire only when
  nothing has focus (verified in Fyne v2.7.3, `internal/driver/glfw/window.go`).
- A focused `KeySink` forwards all four event types to `KeyManager`. Widgets
  that legitimately own text input (entries) consume events themselves and
  forward to `KeyManager` only what the active handler needs (for example the
  conflict dialog name entry forwards `KeyDown`/`KeyUp` for modifier tracking).
- `ui_setup.go` registers the canvas-level callbacks as the no-focus fallback.
  Each callback carries a defensive `Focused() != nil` guard so delivery stays
  single per event even if a future Fyne version invokes canvas callbacks
  alongside the focused object.

Input-owner transitions:

- Commands that open a dialog/menu, enter an input mode, or create a new window
  are deferred until all currently pressed keys have been released.
- Dialog/menu/input-mode close paths that return control to the main file list
  use the same central gate before popping their handler.
- While such a transition is pending, `KeyManager` consumes typed key/rune
  events and only uses key-up events to update pressed-key state.
- This keeps the triggering key from leaking into the newly opened input owner
  (for example a history filter field) and prevents late `Return` typed events
  from falling through to the main file list.
- Cursor movement, selection, refresh, and other non-input-owner state changes
  remain immediate so key repeat behavior stays responsive.

Main-screen configurable bindings:

- Configured under `ui.keyBindings` in `config.json`.
- Key specs support forms such as `C-N`, `S-J`, `S-Q`, `C-S-Q`, `C-S-F`, `A-X`, `F2`, `Return`,
  and `Delete`.
- Modifiers are limited to `S`, `A`, and `C`; unknown modifiers or key names are
  logged as warnings and that binding entry is ignored.
- Optional event values are `typed`, `down`, and `up`. When omitted, modifier
  bindings default to `down`; unmodified bindings default to `typed`.
- User bindings are evaluated before built-in defaults, so a configured binding
  for the same key/event overrides the default behavior.
- Optional `init.star` configuration can append bindings and register `user.*`
  commands. Starlark command functions receive key/modifier context and may call
  `nmf.run(command_id)` to compose built-in or custom commands.

## Focus Ownership Rules

Main file list:

- Wrap list with `ui.KeySink`.
- Keep focus on sink to ensure all keyboard events route through `KeyManager`.
- Enable tab capture (`WithTabCapture(true)`) to suppress default focus traversal.
- When debug logging is enabled, the toolbar includes a mouse action that writes
  `KeyManager.DumpState()` to the debug log without opening another input owner.

Text entries that must not steal Tab:

- Use `ui.TabEntry` (`AcceptsTab` aware entry wrapper).

## Dialog Handler Lifecycle Pattern

Required sequence for keyboard-driven dialogs:

1. Create dialog-specific key handler.
2. `PushHandler` before showing dialog and keep the returned `HandlerToken`.
3. Wrap content with `KeySink` and focus it.
4. On all close paths, `RemoveHandler(token)` once. The token identifies the
   dialog's own stack entry, so an out-of-order or duplicate removal cannot
   evict another owner's handler; such calls only log a warning.
5. Optionally call parent `Canvas().Unfocus()` to avoid stale focus targets.

This pattern is used in history/filter/tree/directory-jump/copy-move/jobs/quit dialogs.

Built-in file viewer:

- `viewer.show` opens the selected file with the internal viewer and pushes
  `FileViewerKeyHandler` while the dialog is open.
- The handler owns less-like navigation keys (`j/k`, `f/b`, `g/G`, `n/N`,
  `/`, `:`, `q`) so keys do not fall through to the main file list if focus
  moves to non-text parts of the dialog.
- The Text and hex panes use a TextGrid PoC that renders only visible text for
  faster initial display. It supports less-like vertical movement, line jumps,
  horizontal movement, a wrap toggle, mouse drag selection, copy, and literal
  current-match search. Keyboard selection is intentionally not wired yet.
- Markdown files open on the Text pane by default. The Markdown tab remains
  available for manual rendered preview.
- Search and line inputs are normal entries; submitted searches return focus to
  the active viewer pane regardless of match result.

Filter-style text input:

- Dialogs that edit search text through key handlers remove the last UTF-8 rune
  for Backspace and `Ctrl-H`; they must not trim by byte.
- Navigation History, Apply Filter, Incremental Search, and Copy/Move
  destination search build matchers through `internal/search`, which provides
  whitespace-separated AND matching with substring and optional embedded migemo
  expansion per token.
- Apply Filter stores comments after `;;` with the filter history entry. The
  comment is searchable, while the applied glob is only the text before `;;`.
- Navigation History and Apply Filter use `Ctrl+Enter` to apply the current
  input directly. Apply Filter uses `Ctrl+D` to delete the selected history
  entry; Navigation History uses `Ctrl+D` to unpin a saved path.
- Directory Jump keeps shortcut-prefix matching separate from migemo so its
  unique-match auto-jump behavior stays deterministic.

Line edit dialogs:

- `F2` or `R` opens a single-item rename dialog for the cursor row only.
- `C-L` opens a path edit dialog instead of focusing the path display directly.
- The focused entry owns normal text input and standard Entry editing; the dialog key handler commits with Enter, cancels with Escape, and adds readline-style Ctrl-A/E/B/F/H/D/K/U editing.

Rename behavior:

- Rename is a direct same-directory operation and does not use the copy/move job queue.

Compare dialog:

- `S-C` opens a direct-directory compare dialog through `compare.show`.
- The source is the focused File Manager's current directory. Opening the dialog
  clears any active file filter so all direct files are compared.
- The destination picker reuses the same history/open-window candidate model as
  Copy/Move, and the accepted comparison replaces the current mark set.

Delete dialogs:

- `Delete` opens a confirmation dialog that queues a trash/recycle-bin job.
- `Shift+Delete` opens a stronger confirmation dialog that requires typing
  `DELETE` before queueing a permanent delete job.
- Dialog handlers must pop exactly once on confirm, cancel, or close.

## Busy State Behavior

When directory loading enters busy mode:

- Push `BusyKeyHandler` to consume input during critical section.
- Pop it after load completes.

## Invariants

- Avoid attaching ad-hoc key handling directly to arbitrary widgets when a `KeyManager` handler should own behavior.
- Keep handler push/pop balanced across success, cancel, and error close paths.
- Do not add per-dialog "skip first typed rune" guards for opener keys. Input-owner
  transitions must rely on the central `KeyManager` transition gate so the next
  real typed character is accepted by the new owner.
- For list cursor UX, unselect default list selection and keep a single visual cursor model.
