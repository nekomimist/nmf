package main

import (
	"time"

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
	stepStart := time.Now()
	preview, err := fileinfo.ReadPreviewFileWithDebug(file.Path, debugPrint)
	debugPrint("FileViewer: read-preview elapsed=%s path=%s err=%v", time.Since(stepStart), file.Path, err)
	if err != nil {
		debugPrint("FileViewer: open failed path=%s err=%v", file.Path, err)
		ui.ShowMessageDialog(fm.window, "Viewer failed", err.Error())
		fm.FocusFileList()
		return
	}

	dialog := ui.NewFileViewerDialog(preview, fm.keyManager)
	dialog.SetDebugPrint(debugPrint)
	stepStart = time.Now()
	dialog.ShowDialog(fm.window)
	debugPrint("FileViewer: show-dialog elapsed=%s", time.Since(stepStart))
	debugPrint("FileViewer: open-ready elapsed=%s", time.Since(totalStart))
}
