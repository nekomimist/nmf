package main

import (
	"time"

	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// ShowFileViewer opens the selected file in the built-in preview viewer.
func (fm *FileManager) ShowFileViewer() {
	totalStart := time.Now()
	currentIdx := fm.GetCurrentCursorIndex()
	files := fm.GetFiles()
	if currentIdx < 0 || currentIdx >= len(files) {
		return
	}

	file := files[currentIdx]
	if file.Name == ".." || file.IsDir {
		return
	}

	debugPrint("FileViewer: open-start path=%s", file.Path)
	viewerID := fm.beginViewerLoad()
	fm.beginBusy("Opening preview...", func() {
		if fm.invalidateViewerLoad(viewerID) {
			fm.endBusy()
			fm.FocusFileList()
		}
	})
	go func() {
		stepStart := time.Now()
		preview, err := fileinfo.ReadPreviewFileWithDebug(file.Path, debugPrint)
		debugPrint("FileViewer: read-preview elapsed=%s path=%s err=%v", time.Since(stepStart), file.Path, err)
		fyne.Do(func() {
			if fm.isWindowClosed() || !fm.finishViewerLoad(viewerID) {
				return
			}
			fm.endBusy()
			if err != nil {
				debugPrint("FileViewer: open failed path=%s err=%v", file.Path, err)
				fm.ShowMessageDialog("Viewer failed", err.Error())
				fm.FocusFileList()
				return
			}

			dialog := ui.NewFileViewerDialog(preview, fm.keyManager)
			dialog.SetMaxSize(fm.config.UI.Viewer.MaxWidth, fm.config.UI.Viewer.MaxHeight)
			dialog.SetDefaultPane(fm.config.UI.Viewer.DefaultPane)
			dialog.SetKeyBindings(fm.config.UI.KeyBindings)
			dialog.SetDebugPrint(debugPrint)
			stepStart = time.Now()
			dialog.ShowDialog(fm.window)
			debugPrint("FileViewer: show-dialog elapsed=%s", time.Since(stepStart))
			debugPrint("FileViewer: open-ready elapsed=%s", time.Since(totalStart))
		})
	}()
}
