package main

import (
	"strings"

	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// ShowRenameDialog shows a direct single-item rename dialog.
func (fm *FileManager) ShowRenameDialog() {
	idx := fm.GetCurrentCursorIndex()
	if idx < 0 || idx >= len(fm.files) {
		debugPrint("FileManager: No valid target for rename")
		return
	}

	target := fm.files[idx]
	if target.Name == ".." || target.Status == fileinfo.StatusDeleted {
		debugPrint("FileManager: Invalid rename target: %s", target.Name)
		return
	}

	dlg := ui.NewRenameDialog(target.Name, fm.keyManager)
	dlg.ShowDialog(fm.window, func(newName string) bool {
		return fm.renameCurrentFile(target, newName)
	})
}

func (fm *FileManager) renameCurrentFile(target fileinfo.FileInfo, newName string) bool {
	trimmed := strings.TrimSpace(newName)
	if trimmed == target.Name {
		fm.FocusFileList()
		return true
	}

	newPath, err := fileinfo.RenamePortable(target.Path, trimmed)
	if err != nil {
		debugPrint("FileManager: Rename failed %s -> %s: %v", target.Path, trimmed, err)
		ui.ShowMessageDialog(fm.window, "Rename failed", err.Error())
		return false
	}

	fm.applyRenameToList(target.Path, trimmed, newPath)
	debugPrint("FileManager: Renamed %s -> %s", target.Path, newPath)
	fm.FocusFileList()
	return true
}

func (fm *FileManager) applyRenameToList(oldPath, newName, newPath string) {
	fm.mu.Lock()
	updated := false
	for i := range fm.files {
		if fm.files[i].Path == oldPath {
			fm.files[i].Name = newName
			fm.files[i].Path = newPath
			fm.files[i].Status = fileinfo.StatusNormal
			fm.cursorPath = newPath
			updated = true
			break
		}
	}
	for i := range fm.originalFiles {
		if fm.originalFiles[i].Path == oldPath {
			fm.originalFiles[i].Name = newName
			fm.originalFiles[i].Path = newPath
			fm.originalFiles[i].Status = fileinfo.StatusNormal
			break
		}
	}

	if fm.selectedFiles[oldPath] {
		delete(fm.selectedFiles, oldPath)
		fm.selectedFiles[newPath] = true
	}

	if !updated {
		fm.mu.Unlock()
		return
	}

	items := make([]interface{}, 0, len(fm.files))
	for i, file := range fm.files {
		items = append(items, fileinfo.ListItem{Index: i, FileInfo: file})
	}
	fm.fileBinding.Set(items)
	fm.fileList.Refresh()
	fm.RefreshCursor()
	fm.mu.Unlock()

	if fm.dirWatcher != nil {
		fm.dirWatcher.RefreshSnapshot()
	}
}
