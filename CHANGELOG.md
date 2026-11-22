# Changelog

## Unreleased

## 0.1.9 - 2026-11-22

- Repeatedly opening the copy or move dialog uses less memory with a long
  destination history.

## 0.1.8 - 2026-11-22

- Cursor memory keeps the position just saved when directories share a
  last-used timestamp and the history is full.

- Debug mode saves a baseline Go heap profile at startup. Click the debug
  toolbar action to log memory usage and capture another profile for comparison.

- The image viewer releases decoded images for garbage collection when you
  close the viewer or switch files, even if the window stays idle.

## 0.1.7 - 2026-11-21

- Jobs shows the full paths of successfully deleted or trashed items, even
  when a batch fails or is canceled. Successful renames appear in Jobs history
  with both the original and new full paths.
- Compare Directories can mark subdirectories missing from the destination.
  The dialog also opens when the current directory has no files.
- Create Directory and Create Text File remember their last 20 names across
  restarts. Use Up/Down or Ctrl+P/Ctrl+N to recall a name.

## 0.1.6 - 2026-11-20

- Hidden files remain visible by default. **Breaking:** an explicit
  `ui.showHiddenFiles: false` now hides dotfiles and entries with the Windows
  hidden attribute. Remove that setting or set it to `true` to keep them visible.
- Copy, move, rename and delete preserve the exact selected filename, including
  trailing whitespace. Renaming a conflict now checks the new destination's
  type and modification time before applying an overwrite decision.
- Copy and extraction populate read-only directories before applying their
  final permissions and timestamps. Directory comparison reports unreadable
  entries and partial read failures.
- Keep password keystrokes out of debug logs, and omit credential-bearing SMB
  addresses from path entry, configured directory jumps and startup errors.
- Opening files and checking selections for extraction keep the window
  responsive. Remote archive detection reads at most 64 KiB. Archives with
  unusual extensions support navigation and parent jumps.
- Canceling a preview or transfer interrupts direct SMB I/O. An interrupted
  SMB replacement attempts to restore the previous destination and reports the
  backup location if recovery fails.
- Directory watches catch changes made during navigation and converge after
  event bursts. Creating or renaming an entry respects the active listing
  order and filter. Each window restores and toggles its own filter.
- Collecting marked files for a drag, restoring the cursor after a refresh and
  updating the Jobs window scale better with large listings. Moving a directory also updates
  pinned and historical descendants across local and SMB paths.
- Long text lines and hex previews render the visible range on demand.
  BOM-marked UTF-16 text opens in the Text pane, and backward search wraps
  correctly from the first match. Tree navigation and filtering long lists of
  destinations also reuse previously computed rows.

## 0.1.5 - 2026-11-17

- Show effective key bindings for Main, Line Edit, and File Viewer with F1,
  including command descriptions and default/custom assignments. Close help
  to resume the previous input or preview.
- Scroll wrapped Text, Markdown, and Hex previews by displayed rows, including
  within long lines, and limit paging to one row in a one-row viewport.
- Reveal search matches in wrapped lines even when they occur beyond the first
  screen of the line.

## 0.1.4 - 2026-11-15

- Switch destination lists in Copy, Move, Extract, and Compare Directories
  between Frecency and Last Access order using the order radio or Ctrl+R.
- Serialize RGBA theme colors as JSON arrays instead of Base64 strings so they
  retain their values when encoded and decoded.

## 0.1.3 - 2026-10-27

- Show the total size of marked files after the status bar's mark count when
  entries are marked, counting directories as zero bytes.

## 0.1.2 - 2026-09-27

- Show Starlark command errors in a dismissible dialog with the source location,
  call history, and a Copy details action, even when debug logging is disabled.

## 0.1.1 - 2026-09-22

- Install and update Windows x64 and ARM64 builds through the
  [nekomimist Scoop bucket](https://github.com/nekomimist/scoop-bucket).
- Refresh the file list immediately after changing sort order, including from
  Starlark menus, when the cursor stays on `..` or another unchanged row.

## 0.1.0 - 2026-09-18

- Establish the first Semantic Versioning release, with the existing keyboard-driven
  file manager, local/SMB/archive browsing, background file operations, and
  multi-window support.
- Rename the main window from "File Manager" to "Nekomimist Filer".
  **Breaking:** update window-title matching in personal automation scripts.
- Provide Windows x64 and ARM64 ZIP downloads with license documents on GitHub
  Releases.
