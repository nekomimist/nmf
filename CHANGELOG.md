# Changelog

## Unreleased

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
