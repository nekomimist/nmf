# UI Input and Focus Model

## Keyboard Routing Architecture

Source packages:

- `internal/keymanager`
- `internal/ui/key_sink.go`
- `internal/ui/tab_entry.go`

Core model:

- `KeyManager` maintains a stack of handlers.
- Active handler is the top of stack.
- The main-screen handler maps key events to stable internal command IDs before
  executing file-manager behavior.
- Events routed by type:
  - `OnKeyDown`
  - `OnKeyUp`
  - `OnTypedKey`
  - `OnTypedRune`

Modifier keys (`Shift`, `Ctrl`, `Alt`) are tracked centrally in `KeyManager` and passed to handlers.

Main-screen configurable bindings:

- Configured under `ui.keyBindings` in `config.json`.
- Key specs support forms such as `^N`, `S-J`, `C-S-F`, `A-X`, `F2`, `Return`,
  and `Delete`.
- Optional event values are `typed`, `down`, and `up`. When omitted, modifier
  bindings default to `down`; unmodified bindings default to `typed`.
- User bindings are evaluated before built-in defaults, so a configured binding
  for the same key/event overrides the default behavior.

## Focus Ownership Rules

Main file list:

- Wrap list with `ui.KeySink`.
- Keep focus on sink to ensure all keyboard events route through `KeyManager`.
- Enable tab capture (`WithTabCapture(true)`) to suppress default focus traversal.

Text entries that must not steal Tab:

- Use `ui.TabEntry` (`AcceptsTab` aware entry wrapper).

## Dialog Handler Lifecycle Pattern

Required sequence for keyboard-driven dialogs:

1. Create dialog-specific key handler.
2. `PushHandler` before showing dialog.
3. Wrap content with `KeySink` and focus it.
4. On all close paths, `PopHandler` once.
5. Optionally call parent `Canvas().Unfocus()` to avoid stale focus targets.

This pattern is used in history/filter/tree/directory-jump/copy-move/jobs/quit dialogs.

Rename dialog:

- `F2` or `R` opens a single-item rename dialog for the cursor row only.
- The focused filename entry owns normal text editing; the dialog key handler only commits with Enter and cancels with Escape.
- Rename is a direct same-directory operation and does not use the copy/move job queue.

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
- For list cursor UX, unselect default list selection and keep a single visual cursor model.
