package main

import (
	"errors"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
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
		if x, y, ok := fm.cursorMenuClientPosition(); ok {
			err = shellmenu.ShowAtClientPosition(winCtx.HWND, paths, x, y)
			return
		}
		debugPrint("FileManager: Cursor row anchor unavailable; using mouse position")
		err = shellmenu.Show(winCtx.HWND, paths)
	})
	if err != nil && !errors.Is(err, shellmenu.ErrUnsupported) {
		debugPrint("FileManager: Explorer context menu failed: %v", err)
	}

	fm.FocusFileList()
	fm.refreshDirectoryAfterShellMenu()
}

func (fm *FileManager) cursorMenuClientPosition() (int, int, bool) {
	anchor := fm.cursorAnchor
	if anchor.object == nil || anchor.path == "" || anchor.path != fm.cursorPath {
		return 0, 0, false
	}

	canvas := fyne.CurrentApp().Driver().CanvasForObject(anchor.object)
	if canvas == nil {
		return 0, 0, false
	}

	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor.object)
	size := anchor.object.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return 0, 0, false
	}

	clientPos := pos.AddXY(8, size.Height/2)
	x, y := canvas.PixelCoordinateForPosition(clientPos)
	return x, y, true
}

func (fm *FileManager) refreshDirectoryAfterShellMenu() {
	path := fm.currentPath
	time.AfterFunc(10*time.Millisecond, func() {
		fyne.Do(func() {
			if fm.currentPath != path {
				return
			}
			fm.SaveCursorPosition(path)
			fm.LoadDirectory(path)
		})
	})
}
