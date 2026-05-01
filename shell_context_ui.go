package main

import (
	"errors"
	"runtime"

	"fyne.io/fyne/v2/driver"

	"nmf/internal/fileinfo"
	"nmf/internal/shellmenu"
)

// ShowExplorerContextMenu opens the platform shell context menu for the
// selected files, or the cursor item when no files are selected.
func (fm *FileManager) ShowExplorerContextMenu() {
	if runtime.GOOS != "windows" {
		debugPrint("FileManager: Explorer context menu unsupported on %s", runtime.GOOS)
		return
	}

	paths := fm.collectTargetPaths()
	if len(paths) == 0 {
		debugPrint("FileManager: No valid target for Explorer context menu")
		return
	}
	for _, p := range paths {
		if fileinfo.IsArchivePath(p) {
			debugPrint("FileManager: Explorer context menu unsupported for archive path=%s", p)
			return
		}
	}

	nativeWindow, ok := fm.window.(driver.NativeWindow)
	if !ok {
		debugPrint("FileManager: Native window context unavailable for Explorer context menu")
		return
	}

	var err error
	nativeWindow.RunNative(func(context any) {
		winCtx, ok := context.(driver.WindowsWindowContext)
		if !ok || winCtx.HWND == 0 {
			err = shellmenu.ErrUnsupported
			return
		}
		err = shellmenu.Show(winCtx.HWND, paths)
	})
	if err != nil && !errors.Is(err, shellmenu.ErrUnsupported) {
		debugPrint("FileManager: Explorer context menu failed: %v", err)
	}

	fm.FocusFileList()
	fm.SaveCursorPosition(fm.currentPath)
	fm.LoadDirectory(fm.currentPath)
}
