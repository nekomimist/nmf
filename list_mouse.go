package main

import (
	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
)

func (fm *FileManager) handleFileNameClick(index int, clicked fileinfo.FileInfo, modifier fyne.KeyModifier) {
	if fm.selectedFiles == nil {
		fm.selectedFiles = make(map[string]bool)
	}
	anchor := fm.GetCurrentCursorIndex()
	fm.SetCursorByIndex(index)

	if modifier&fyne.KeyModifierShift != 0 {
		fm.markFileRange(anchor, index)
	} else if isTargetFileInfo(clicked) {
		fm.selectedFiles[clicked.Path] = !fm.selectedFiles[clicked.Path]
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
	if len(fm.files) == 0 {
		return
	}
	if anchor < 0 || anchor >= len(fm.files) {
		anchor = target
	}
	if target < 0 || target >= len(fm.files) {
		return
	}
	start, end := anchor, target
	if start > end {
		start, end = end, start
	}

	changed := false
	for i := start; i <= end; i++ {
		fi := fm.files[i]
		if !isTargetFileInfo(fi) || fm.selectedFiles[fi.Path] {
			continue
		}
		fm.selectedFiles[fi.Path] = true
		changed = true
	}
	if changed {
		fm.updateStatusBar()
	}
}
