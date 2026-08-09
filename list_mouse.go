package main

import (
	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
)

func (fm *FileManager) handleFileNameClick(index int, clicked fileinfo.FileInfo, modifier fyne.KeyModifier) {
	anchor := fm.GetCurrentCursorIndex()
	fm.SetCursorByIndex(index)

	if modifier&fyne.KeyModifierShift != 0 {
		fm.markFileRange(anchor, index)
	} else if isTargetFileInfo(clicked) {
		fm.browserModel().ToggleSelected(clicked.Path)
		fm.updateStatusBar()
	}

	if fm.fileList != nil {
		fm.fileList.UnselectAll()
	}
	fm.FocusFileList()
	if fm.fileList != nil {
		fm.RefreshCursor()
	}
}

func (fm *FileManager) markFileRange(anchor, target int) {
	count := fm.FileCount()
	if count == 0 {
		return
	}
	if anchor < 0 || anchor >= count {
		anchor = target
	}
	if target < 0 || target >= count {
		return
	}
	if fm.browserModel().MarkRange(anchor, target) {
		fm.updateStatusBar()
	}
}
