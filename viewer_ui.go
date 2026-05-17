package main

import (
	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// ShowFileViewer opens the selected file in the built-in preview viewer.
func (fm *FileManager) ShowFileViewer() {
	currentIdx := fm.GetCurrentCursorIndex()
	files := fm.GetFiles()
	if currentIdx < 0 || currentIdx >= len(files) {
		return
	}

	file := files[currentIdx]
	if file.Name == ".." || file.IsDir {
		return
	}

	preview, err := fileinfo.ReadPreviewFile(file.Path)
	if err != nil {
		debugPrint("FileViewer: open failed path=%s err=%v", file.Path, err)
		ui.ShowMessageDialog(fm.window, "Viewer failed", err.Error())
		fm.FocusFileList()
		return
	}

	dialog := ui.NewFileViewerDialog(preview)
	dialog.ShowDialog(fm.window)
}
