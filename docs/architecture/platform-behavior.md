# Platform Behavior

## Scope

NMF targets Windows and Linux at the UI and file-operation layer, but some
desktop integrations depend on platform-native APIs. Shared Unix code is
compile-checked for Darwin, but macOS has not been manually validated and is
not currently a supported release target. This page records the intended
behavior and the supported platform surface for those integrations.

## Summary

| Feature | Windows | Linux | macOS |
| --- | --- | --- | --- |
| Directory listing, metadata, copy, move, rename, delete | Supported through portable file APIs | Supported through portable file APIs | Core compiles; GUI behavior is unverified |
| UNC/SMB navigation | `\\server\share` and `smb://...` resolve to native UNC/local-provider access | `smb://...` and `//server/share` prefer mounted shares, then Linux direct SMB | Direct SMB is unsupported |
| External files dropped onto NMF | Supported through Fyne `Window.SetOnDropped` | Supported when the desktop backend provides file URIs | Unverified Fyne behavior |
| Dragging files from NMF to another app | Supported through Windows Shell `IDataObject` and `DoDragDrop` | Not implemented | Not implemented |
| Explorer/shell context menu | Supported through Windows Shell context menu APIs | Not implemented | Not implemented |
| New File Manager placement beside source window | Supported through Win32 `HWND` positioning | Uses the window manager's default placement | Uses the window manager's default placement; unverified |
| Raise visible File Manager windows together | Optional through Win32 Z-order positioning | Not implemented | Not implemented |
| File Manager focus switching with Left/Right | Uses Win32 `HWND` window positions | Uses creation order on X11; unsupported on Wayland because the compositor controls focus activation | Unverified |
| Native file icons | Uses Windows shell icons through the icon service | Uses theme/generic icons | Uses theme/generic icons; unverified |

## Windows RDP and DPI Detection

NMF previously set `FYNE_DISABLE_DPI_DETECTION=1` during Windows startup as a
defensive workaround for a Fyne/GLFW monitor re-detection failure when an RDP
session changed the display topology. Fyne 2.8.1 guards failed video-mode
queries when a monitor disappears, so NMF leaves per-monitor DPI detection
enabled by default while that upstream fix is validated.

The previous workaround remains behind `forceDisableFyneDPIDetection` in
`dpi_workaround_windows.go` for quick RDP regression testing. Setting that
constant to `true` restores the startup environment override before the primary
display probe and first Fyne window are created. Users can also set
`FYNE_DISABLE_DPI_DETECTION=1` externally without changing the build.

## SMB and UNC Paths

SMB display paths are canonicalized as `smb://host/share/...` in the UI and
history. Windows UNC input such as `\\server\share\path` is normalized into
that display form, but Windows I/O resolves it back to native UNC access. WSL
aliases such as `\\wsl$`, `//wsl$/`, and `smb://wsl$/` are recorded as
`smb://wsl.localhost/...`.

Detailed provider selection and job behavior lives in `vfs-smb.md`.

Current platform policy:

- Windows resolves UNC and `smb://` through `LocalFS` over native UNC paths.
- Linux resolves mounted SMB/CIFS shares to local mount paths when possible.
- Linux falls back to the direct SMB provider when no matching mount exists.
- Direct SMB is unsupported outside Linux and Windows' native UNC path. Those
  platforms return an explicit error rather than mapping the SMB-relative path
  onto `LocalFS`.
- Same-directory rename is no-clobber on Windows, Linux, and Darwin. Where the
  filesystem does not implement flagged rename -- 9p/drvfs, vfat, exfat, some
  network mounts, and any non-APFS volume on macOS -- Linux and Darwin degrade
  to a plain rename guarded by an immediate existence check rather than failing
  the operation, since those are exactly the removable and Windows-hosted
  volumes users browse. See `docs/architecture/vfs-smb.md` for the contract.
- Other Unix/BSD builds currently return an explicit unsupported error because
  their portable `rename` operation may replace a destination; NMF does not
  fall back to the previous racy check-then-overwrite behavior.

## Desktop Drop Target

Incoming file drops are registered in `drop_ui.go` through
`fyne.Window.SetOnDropped`.

Behavior:

- Dropped `file://` URIs are resolved to local/native paths.
- The current File Manager directory is used as the destination.
- NMF prompts for copy or move before queuing jobs.
- Windows UNC-backed current directories are valid destinations because they
  resolve through native UNC/local-provider access.

Linux/Unix support depends on the desktop environment and Fyne backend
providing file URIs. No Linux-specific desktop drop protocol handling is
implemented beyond Fyne's drop callback.

## Desktop Drag Source

Outbound file drag is implemented for Windows only:

- UI trigger: the file icon and file name widgets record `MouseDown` and start
  the drag from `MouseMoved` after a distance threshold.
- File collection and validation: `drag_source_ui.go`.
- Native Shell drag loop: `internal/shellmenu.StartFileDrag` in
  `shellmenu_windows.go`.
- Non-Windows stub: `shellmenu_other.go`.

Only copy effects are advertised to the Shell, so NMF does not remove source
files as part of a drag operation.

The drag trigger intentionally avoids Fyne's `Draggable` path. In testing,
starting the Windows Shell `DoDragDrop` loop from Fyne's drag callback left
later mouse interactions unreliable even after the Shell drag completed.

Unsupported sources:

- archive entries
- deleted/status-only entries
- direct SMB provider items that do not resolve to local/native paths

## Native Shell Context Menu

Explorer context menus are Windows-only:

- UI entrypoint: `shell_context_ui.go`.
- Native implementation: `internal/shellmenu` with Windows Shell
  `IContextMenu` APIs.
- Non-Windows behavior: returns `shellmenu.ErrUnsupported`.

Other platforms do not currently provide an equivalent native file-manager
context menu integration.

## Window Placement

`Ctrl-N` opens a second File Manager window.

- Windows places the new window beside the source window using
  `driver.NativeWindow`, Win32 `HWND`, and `SetWindowPos` in
  `window_position_windows.go`.
- If another File Manager window already occupies the preferred side within a
  small left/top coordinate threshold, Windows placement tries the opposite side
  when there is enough work-area space; if neither side is available, it uses
  the same clamped fallback position as the no-space case.
- When `window.moveSourceOnNewWindow` is enabled, the source is the only
  existing File Manager, and neither side has enough room at its current
  position, NMF compares the horizontal movement needed for a left or right
  pair. If both outer widths fit the source monitor work area, it moves the
  source by the smaller amount and places the new window beside it; ties prefer
  the right. Maximized, minimized, snapped, or otherwise unclassifiable source
  windows are not moved. The setting defaults to `false` during evaluation.
- With two or more existing File Managers, NMF keeps the source fixed and
  retains the right, left, then clamped 32-pixel-offset fallback.
- Other platforms intentionally use default window-manager placement through
  `window_position_other.go`.

## Window Activation Grouping

On Windows, `window.bringAllToFront` optionally groups File Manager windows in
the Z order when one becomes active. NMF verifies that the foreground native
window is a registered File Manager, leaves it active, and moves each other
visible, non-minimized File Manager directly behind it in its existing relative
order. The operation does not activate siblings, restore minimized windows,
include the Jobs window, or mark any window as always-on-top. Other platforms
ignore this setting.

## Window Focus Switching

The main screen binds `Left` and `Right` to switch between File Manager windows
inside the same NMF process.

- Windows chooses the nearest File Manager window to the left or right using
  Win32 window rectangles.
- X11/other non-Wayland desktops use File Manager creation order.
- Wayland does not allow an application to focus an existing top-level window
  programmatically without compositor-mediated user activation, and Fyne's GLFW
  driver intentionally leaves `RequestFocus` as a no-op on Wayland. NMF logs the
  selected target but does not attempt a misleading focus request there.
- Manual verification so far covers Windows. Linux X11 and XWayland behavior
  still needs confirmation on a desktop that can run the X11 build, because the
  current Linux test environment is WSLg/Wayland-only.

## Window Visual State

Copy/move, directory-comparison, and navigation-history dialogs can highlight
every File Manager window, including the dialog's own window, that displays the
currently selected open-directory candidate. This is an in-content overlay
border, not a native OS window decoration. A fixed 4 logical-pixel safe inset
keeps the frame clear of main content, dialog controls, and list scrollbars.
On Windows, iconified File Manager windows are skipped using the native
`IsIconic` state; other platforms do not currently expose iconified state
through Fyne and use the normal overlay path.
When the selected path belongs to the dialog's own File Manager, the dialog
content also uses the same frame above Fyne's modal dim layer so the accent
remains clear; the dimmed File Manager frame remains active underneath.

Inactive File Manager windows dim only the file-list cursor. Fyne does not expose
a public top-level window activation callback, so NMF derives this from the main
File Manager `KeySink` focus state.

## Adding Platform Integrations

When adding a platform-specific feature:

- Keep OS-specific code behind Go build tags.
- Provide an explicit unsupported stub for other platforms.
- Route path handling through `internal/fileinfo` resolver and portable APIs.
- Document the platform behavior in this page and link to more detailed
  architecture docs when needed.
