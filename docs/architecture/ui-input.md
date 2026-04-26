# UI Input and Focus Model

## Keyboard Routing Architecture

Source packages:

- `internal/keymanager`
- `internal/ui/key_sink.go`
- `internal/ui/tab_entry.go`

Core model:

- `KeyManager` maintains a stack of handlers.
- Active handler is the top of stack.
- Events routed by type:
  - `OnKeyDown`
  - `OnKeyUp`
  - `OnTypedKey`
  - `OnTypedRune`

Modifier keys (`Shift`, `Ctrl`, `Alt`) are tracked centrally in `KeyManager` and passed to handlers.

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

## Busy State Behavior

When directory loading enters busy mode:

- Push `BusyKeyHandler` to consume input during critical section.
- Pop it after load completes.

## Invariants

- Avoid attaching ad-hoc key handling directly to arbitrary widgets when a `KeyManager` handler should own behavior.
- Keep handler push/pop balanced across success, cancel, and error close paths.
- For list cursor UX, unselect default list selection and keep a single visual cursor model.
