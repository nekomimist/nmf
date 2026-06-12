# UI Input and Focus Model

## Keyboard Routing Architecture

Source packages:

- `internal/keymanager`
- `internal/ui/key_sink.go`
- `internal/ui/tab_entry.go`

Core model:

- `KeyManager` maintains a stack of handlers; the active handler is the top.
- Handlers implement only `OnKeyActivated(ev, modifiers)` and
  `OnTypedRune(r, modifiers)`. An "activation" merges Fyne's TypedKey and
  TypedShortcut deliveries (both repeat while a key is held).
- Raw key down/up never reach handlers. They are KeyManager-internal plumbing
  with three uses: modifier tracking, remembering the physical key behind
  folded standard shortcuts (`lastKeyDown`), and arming the input gate.
- For shortcut-path activations the modifiers come from the shortcut event
  itself; only Shift on the typed-key path relies on tracked state, because
  `fyne.KeyEvent` carries no modifier information and Shift-only combos never
  become shortcuts.
- `DumpState` returns handler stack, modifier state, gate state, queued
  transition count, and `lastKeyDown` for debug logs.
- The main-screen handler maps activations to stable internal command IDs
  before executing file-manager behavior. Each command definition carries a
  `transition` attribute marking it as an input-owner change.

Driver facts this design relies on (verified in Fyne v2.7.3,
`internal/driver/glfw/window.go`; re-verify on Fyne upgrades — see
`docs/todo-keyboard.md` for details):

1. Exclusive delivery: focused object or (only when nothing has focus) the
   canvas-level callbacks.
2. Key repeats produce TypedKey/TypedShortcut but never KeyDown.
3. Ctrl-only C/X/V/A/Z/Y/Insert and Shift-only Insert/Delete are folded into
   standard shortcuts (Copy/Cut/Paste/SelectAll/Undo/Redo); other Ctrl/Alt
   combos arrive as `desktop.CustomShortcut` with exact modifier bits.
   `KeyManager.HandleShortcut` reconstructs the physical combo for folded
   shortcuts from `lastKeyDown`.
4. `fyne.Do` queued from the main thread runs after the current event batch,
   including the trailing TypedRune of the same key press.
5. Window focus loss reaches the focused widget's `FocusLost()`.

Event delivery paths:

- A focused `KeySink` forwards key down/up, typed keys, runes, and all
  shortcuts to `KeyManager`.
- Invariant: every path that forwards activations into the KeyManager must
  also forward key downs, because a fresh press is what arms the gate
  (KeySink, canvas fallback, viewer text grid, conflict dialog name entry).
- `ui_setup.go` registers the canvas-level callbacks as the no-focus fallback
  with defensive `Focused() != nil` guards. Because the driver routes
  shortcuts to the canvas shortcut table in that state, the main screen's
  Ctrl/Alt activations (`MainScreenKeyHandler.ActivationShortcuts`) are also
  registered on the canvas.

Input gating (owner transitions):

- Commands and close paths that change the input owner run through
  `KeyManager.BeginOwnerTransition`: the action is queued onto the next Fyne
  main-loop iteration, so the remaining events of the triggering key press
  (e.g. its TypedRune) are delivered to the old owner and discarded there.
- The gate disarms on handler push/remove, queued transitions, and KeySink
  focus changes; it re-arms on the next fresh non-modifier key down. Since
  repeats never produce key downs, a key held across an owner change can
  never fire into the new owner. The arming press itself is fully delivered.
- Events arriving while disarmed are dropped, never queued.
- `ResetTransientState` (focus changes, external-open failures) clears
  modifiers, `lastKeyDown`, and the gate, so stale Shift state cannot
  survive an alt-tab.
- Cursor movement, selection, refresh, and other non-transition commands
  remain immediate, and every binding repeats while its key is held.

Main-screen configurable bindings:

- Configured under `ui.keyBindings` in `config.json`.
- Key specs support forms such as `C-N`, `S-J`, `S-Q`, `C-S-Q`, `C-S-F`, `A-X`, `F2`, `Return`,
  and `Delete`.
- Modifiers are limited to `S`, `A`, and `C`; unknown modifiers or key names are
  logged as warnings and that binding entry is ignored.
- Bindings fire on activation; whether that is the typed-key or shortcut path
  is derived from the key spec. The legacy `event` field (`typed`/`down`/`up`)
  is deprecated: it is accepted but ignored with a warning.
- User bindings are evaluated before built-in defaults, so a configured binding
  for the same key overrides the default behavior.
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
