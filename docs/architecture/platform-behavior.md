# Platform Behavior

## Scope

NMF is cross-platform at the UI and file-operation layer, but some desktop
integrations depend on platform-native APIs. This page records the intended
behavior and the supported platform surface for those integrations.

## Summary

| Feature | Windows | Linux/Unix |
| --- | --- | --- |
| Directory listing, metadata, copy, move, rename, delete | Supported through portable file APIs | Supported through portable file APIs |
| UNC/SMB navigation | `\\server\share` and `smb://...` resolve to native UNC/local-provider access | `smb://...` and `//server/share` prefer mounted shares, then Linux direct SMB |
| External files dropped onto NMF | Supported through Fyne `Window.SetOnDropped` | Supported through Fyne `Window.SetOnDropped` when the desktop backend provides file URIs |
| Dragging files from NMF to another app | Supported through Windows Shell `IDataObject` and `DoDragDrop` | Not implemented |
| Explorer/shell context menu | Supported through Windows Shell context menu APIs | Not implemented |
| New File Manager placement beside source window | Supported through Win32 `HWND` positioning | Uses the window manager's default placement |
| File Manager focus switching with Left/Right | Uses Win32 `HWND` window positions | Uses creation order on X11; unsupported on Wayland because the compositor controls focus activation |
| Native file icons | Uses Windows shell icons through the icon service | Uses theme/generic icons |

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
- Non-Linux direct SMB provider parity remains unresolved outside Windows'
  native UNC path.

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

- UI trigger: `TappableIcon` records `MouseDown` and starts the drag from
  `MouseMoved` after a distance threshold.
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
- Other platforms intentionally use default window-manager placement through
  `window_position_other.go`.

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

## Adding Platform Integrations

When adding a platform-specific feature:

- Keep OS-specific code behind Go build tags.
- Provide an explicit unsupported stub for other platforms.
- Route path handling through `internal/fileinfo` resolver and portable APIs.
- Document the platform behavior in this page and link to more detailed
  architecture docs when needed.
