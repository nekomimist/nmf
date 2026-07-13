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

Driver facts this design relies on (verified in Fyne v2.7.3; re-verify these
on Fyne upgrades at the named locations in the Fyne source):

1. Exclusive delivery: focused object or (only when nothing has focus) the
   canvas-level callbacks
   (`internal/driver/glfw/window.go` `processKeyPressed`).
2. Key repeats produce TypedKey/TypedShortcut but never KeyDown; KeyUp is the
   only release-time event (`processKeyPressed`: press/repeat/release paths).
3. Ctrl-only C/X/V/A/Z/Y/Insert and Shift-only Insert/Delete are folded into
   standard shortcuts (Copy/Cut/Paste/SelectAll/Undo/Redo); other Ctrl/Alt
   combos arrive as `desktop.CustomShortcut` with exact modifier bits, and
   Shift-only combos never become shortcuts
   (`internal/driver/glfw/window.go` `triggersShortcut`).
   `KeyManager.HandleShortcut` reconstructs the physical combo for folded
   shortcuts from `lastKeyDown`.
4. `fyne.Do` queued from the main thread runs after the current event batch,
   including the trailing TypedRune of the same key press
   (`internal/driver/glfw/loop.go` `runOnMainWithWait` + run loop).
5. Window focus loss reaches the focused widget's `FocusLost()`
   (`internal/app/focus_manager.go` `FocusManager.FocusLost`).
6. A printable-key press delivers both a TypedKey and a separate TypedRune to
   the focused object (`internal/driver/glfw/window.go` `processKeyPressed`
   and `processCharInput`). Any handler that matches key bindings from both
   callbacks must assign each key spec to exactly one of the two paths;
   matching the same binding list on both makes one press fire twice.
7. `widget.List.ScrollTo` unconditionally ends with a full `Refresh()`
   (`widget/list.go` `ScrollTo`). `RefreshCursor` relies on this to repaint
   with a single render pass; adding an explicit `Refresh()` next to a
   `ScrollTo` doubles the per-keypress render cost.
8. `ScrollTo`'s offset is clamped against the scroller's *current* content
   size, which only updates during a refresh/layout pass
   (`internal/widget/scroller.go` `updateOffset` resets the offset to zero
   while the stale content still fits the viewport). After replacing the
   list's content, `Refresh()` must run before `ScrollTo` — that pair is
   `refreshListAndCursor` — otherwise the list lays rows out for the
   requested offset while the scroll translation stays at zero and the
   viewport looks empty (observed on Windows with a restored cursor beyond
   the first viewport).

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

Cursor refresh diagnostics:

- Debug logs assign every `RefreshCursor`/`refreshListAndCursor` request a
  `seq`. The cursor row's `UpdateItem` callback acknowledges it through
  `itemUpdateSeq`; this acknowledgement means the row decoration was rebuilt,
  not that the GL frame was presented.
- A changed logical cursor with `itemUpdateSeq < refreshSeq` points to the
  List update path. Equal sequences with stale pixels point after
  `UpdateItem`, toward canvas painting or frame presentation.
- The debug toolbar dump includes both sequences, cursor/list state, focus,
  and canvas/list sizes so multi-window failures can be compared without
  changing focus first.

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

Accepted trade-offs of the activation model:

- Bindings cannot fire on raw key down/up anymore; nothing needed them once
  the leak workarounds became unnecessary.
- Input arriving within roughly one main-loop tick of an owner transition is
  dropped (the old model dropped everything until full key release, so the
  window is strictly smaller).
- A dialog whose entry has Fyne focus receives held-key repeats as normal
  text input, like any OS text field.
- Distinguishing `C-C` from `C-Insert` (and `C-X`/`C-V` from their
  Shift-folded variants) depends on the `lastKeyDown` reconstruction; the
  driver events alone cannot tell them apart.
- On macOS the driver folds Cmd (not Ctrl) combos into standard shortcuts;
  Super-modifier handling needs its own design before any macOS support.

Configurable bindings:

- Configured under `ui.keyBindings` in `config.json`.
- `target` defaults to `main`; supported targets are `main`, `lineEdit`, and
  `fileViewer`.
- Key specs support forms such as `C-N`, `S-J`, `S-Q`, `C-S-Q`, `C-S-F`, `A-X`, `F2`, `Return`,
  and `Delete`.
- Modifiers are limited to `S`, `A`, and `C`; unknown modifiers or key names are
  logged as warnings and that binding entry is ignored.
- Unknown `target` values are warned once at startup and the entry is
  ignored; without that check the entry would be silently filtered out of
  every target before per-entry validation runs. The line-edit and
  file-viewer handlers log their construction-time warnings (invalid key
  spec, unknown command, deprecated `event`) through the KeyManager debug
  printer, same as the main screen.
- Bindings fire on activation; whether that is the typed-key or shortcut path
  is derived from the key spec. The legacy `event` field (`typed`/`down`/`up`)
  is deprecated: it is accepted but ignored with a warning.
- User bindings are evaluated before built-in defaults for the same target, so
  a configured binding for the same key overrides the default behavior.
- Optional `init.star` configuration can append bindings and register `user.*`
  commands for the main target. Line-edit and file-viewer targets bind only
  built-in command IDs.

## Focus Ownership Rules

Long-running UI actions:

- Tree widget datasource callbacks return only cached children. Cache misses
  start a background portable directory read and refresh the tree on the Fyne
  goroutine; `ChildUIDs` and `IsBranch` never perform VFS I/O. Platform-root
  branch accessibility (including Windows drive roots) is classified during
  that background read and cached before refresh.
- File preview reads run behind the main busy/input guard on a worker goroutine.
  A per-window viewer generation drops late results after cancellation or
  window close before they can push a viewer handler.
- Direct paths typed into path/history/copy-move/compare dialogs are only
  canonicalized synchronously. Accessibility is checked by the downstream
  asynchronous directory load, job, or comparison, which owns error reporting.
  Copy/move/extract jobs require the destination root to be an existing
  directory; they never create a mistyped destination tree implicitly.

Main file list:

- Wrap list with `ui.KeySink`.
- Keep focus on sink to ensure all keyboard events route through `KeyManager`.
- Enable tab capture (`WithTabCapture(true)`) to suppress default focus traversal.
- When debug logging is enabled, the toolbar includes a mouse action that writes
  `KeyManager.DumpState()` to the debug log without opening another input owner.

Text entries that must not steal Tab:

- Use `ui.TabEntry` (`AcceptsTab` aware entry wrapper).
- One-line edit dialogs use the `lineEdit` target. The copy/move conflict
  dialog's rename entry uses the same line-edit bindings while preserving the
  conflict choice Alt shortcuts; it feeds key down/up into the KeyManager and
  matches its private line-edit handler against the tracked modifier state,
  so Shift-modified keys skip the unmodified bindings and fall through to the
  entry's native selection handling.
- Wrappers that embed an already-extended widget must take the widget impl
  slot themselves: `ExtendBaseWidget` is a no-op once an impl is set, so the
  embedded part is built unextended (`newLineEditEntryForEmbedding` in
  `internal/ui/line_edit_dialog.go`). Otherwise theme lookups and refreshes
  resolve against the embedded object, which is not in the object tree, and
  scoped line-edit theme overrides (cursor/selection colors) silently miss.

## Dialog Handler Lifecycle Pattern

Dialog key dispatch:

- Per-dialog handlers (compare/conflict/copy-move/delete-confirm/directory-jump/
  filter/history/jobs/maintenance/quit/sort/tree, plus incremental search) build
  on a shared base, `dialogKeyHandler` (`internal/keymanager/dialog_handler.go`),
  instead of hand-writing a switch over `ev.Name`/modifiers. Each constructor
  builds a static `[]dialogBinding{ {spec, action}, ... }` table from its
  dialog interface's methods; `spec` uses the same syntax `parseKeySpec`
  accepts for configured key bindings (`Esc`, `Return`, `C-Return`, `S-Up`,
  `1`, ...). Unlike configured bindings, these specs are static string
  literals written by a programmer, so a typo is a construction-time panic
  rather than a warned-and-skipped config entry.
- Matching is exact-modifier, sharing `keySpec.matches` with the configurable
  main-screen/file-viewer/line-edit bindings: a binding fires only when every
  modifier bit matches precisely, same semantics and same "one activation path
  per binding" guardrail. A handler can still attach a rune handler (search/
  filter text entry, sort's `o`/`d` shortcuts) or a fallback invoked when no
  binding matches (the quit dialog uses this to consume every unmatched key so
  nothing leaks to MainScreen).
- File Viewer and Line Edit dialogs keep their own pre-existing declarative
  bindings (`buildTargetKeyBindings`, see "Configurable bindings" above)
  instead of moving onto `dialogKeyHandler`: they are user-configurable via
  `config.json`, which the static per-dialog table intentionally is not.
  `BusyKeyHandler` (swallow-everything guard) also stays a plain struct; its
  entire logic is smaller than a binding table would be.

Required sequence for keyboard-driven dialogs:

1. Create dialog-specific key handler.
2. `PushHandler` before showing dialog and keep the returned `HandlerToken`.
3. Wrap content with `KeySink` and focus it.
4. On all close paths, `RemoveHandler(token)` once. The token identifies the
   dialog's own stack entry, so an out-of-order or duplicate removal cannot
   evict another owner's handler; such calls only log a warning.
5. Optionally call parent `Canvas().Unfocus()` to avoid stale focus targets.

This pattern is used in history/filter/tree/directory-jump/copy-move/jobs/quit dialogs.

Popup dismissal (command menu):

- A non-modal `widget.PopUp` dismisses an outside tap by calling `Hide()`
  directly, and removing an overlay discards its focus manager without
  calling `FocusLost` (verified in Fyne v2.7.3 `widget/popup.go` and
  `internal/overlay_stack.go`; re-verify on Fyne upgrades). A popup that
  relies on those built-in paths never gets a chance to reset input state or
  restore focus when the user clicks elsewhere.
- Popup-style surfaces therefore route every close path, including outside
  taps, through their own `Dismiss()`, which resets KeyManager transient
  state exactly once and runs the dismiss callback. The command menu hosts
  itself in `commandMenuOverlay` (`internal/ui/command_menu.go`), a
  full-canvas overlay that replicates Fyne's `PopUpMenu` overlay-container
  pattern and forwards outside taps to `Dismiss()`.

Dialog sizing:

- Navigation History, Copy/Move, Compare Directories, Tree Dialog, and
  Directory Jump keep their previous fixed widths as minimums, then expand
  horizontally to about 90% of the parent File Manager canvas when opened.
- These dialogs keep their existing fixed heights.
- Rename opts into responsive one-line editing width separately: the dialog
  keeps the default line-edit width as its minimum, expands to about 70% of the
  parent width, and caps at 960px. Other line-edit dialogs stay fixed width.
- The built-in viewer has its own parent-size ratio and
  `viewer.maxWidth`/`viewer.maxHeight` caps.

Dialog button bar:

- Every dialog (and the Jobs window) builds its bottom action row via
  `dialogButtonBar` (`internal/ui/dialog_buttons.go`): buttons are centered as
  a group, each with a minimum-width floor (`dialogButtonMinWidth`) — longer
  labels grow naturally and are never clipped.
- Order is `[auxiliary...] [Cancel/dismiss] [Affirmative, rightmost]`.
  `HighImportance` (blue) marks the default action activated by Enter —
  normally the rightmost affirmative (`ConfirmIcon`); cancel =
  `CancelIcon`+default importance. When the safe action is the Enter default
  (Quit with active jobs), the cancel button carries `HighImportance` instead
  and the destructive affirmative uses `WarningIcon`+`DangerImportance`,
  still rightmost. A lone dismiss with no separate affirmative (Jobs window
  "Close") is the default and styled accordingly.
- Dialogs now uniformly use `dialog.NewCustomWithoutButtons`; the button bar
  lives inside the `KeySink`-wrapped content and calls the same methods the
  keymanager handlers invoke, so keyboard and mouse activation stay in sync.

Built-in file viewer:

- `viewer.show` opens the selected file with the internal viewer and pushes
  `FileViewerKeyHandler` while the dialog is open.
- The handler owns less-like navigation keys (`j/k`, `f/b`, `g/G`, `n/N`,
  `/`, `:`, `q`) and pane switching keys (`t`, `m`, `x`) so keys do not fall
  through to the main file list if focus moves to non-text parts of the dialog.
- These keys are configurable through the `fileViewer` target.
- Because of driver fact 6, viewer bindings are split at construction into a
  typed-key set and a rune set (`fileViewerRunePathSpec` in
  `internal/keymanager/fileviewer_handler.go`): bare and Shift-modified
  letters, unmodified `/`, and Shift+`;` (`:`) fire only on the rune path;
  everything else (arrows, Escape, Space, Ctrl/Alt combos, function keys)
  fires only on the typed-key path. Each binding activates exactly once per
  press.
- The Text, Markdown, and hex panes use a TextGrid PoC that renders only visible text for
  faster initial display. It supports less-like vertical movement, line jumps,
  horizontal movement, a wrap toggle, mouse drag selection, keyboard select-all,
  copy, and literal current-match search. Keyboard range selection is
  intentionally not wired.
- Markdown files open on the Markdown pane by default unless
  `viewer.defaultPane` is set to `text`. The Markdown tab converts
  Markdown AST to simplified text, including fixed-width pipe tables and
  front-matter metadata tables with long cells wrapped to multiple rows, and
  leaves diagram rendering to external viewers.
- Search and line inputs are normal entries with the line-edit cursor and
  selection theme colors. Submitted searches return focus to the active viewer
  pane regardless of match result; Escape returns focus without submitting or
  closing the viewer.

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
- Popup dismissal, including outside taps, must go through the popup's
  `Dismiss()`; never rely on `widget.PopUp`'s built-in outside-tap `Hide()`.
- For list cursor UX, unselect default list selection and keep a single visual cursor model.
- Cursor-only moves end in exactly one Refresh-family call
  (`RefreshCursor()`), per driver fact 7. Operations that replace the files
  slice with different content (load, filter) must use
  `refreshListAndCursor()` instead — its leading `Refresh()` is required by
  driver fact 8, and nothing further may be added after it. Same-length
  mutations (sort, rename, watcher modify) may keep `RefreshCursor()`.
