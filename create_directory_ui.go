package main

import (
	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// ShowCreateDirectoryDialog shows a single-name directory creation dialog.
func (fm *FileManager) ShowCreateDirectoryDialog() {
	dlg := ui.NewLineEditDialog(ui.LineEditDialogOptions{
		Title:       "Create Directory",
		Prompt:      "Directory name:",
		ConfirmText: "Create",
	}, fm.keyManager)
	dlg.ShowDialog(fm.window, func(name string) bool {
		return fm.CreateDirectory(name)
	})
}

// CreateDirectory creates a directory under the current path and selects it.
func (fm *FileManager) CreateDirectory(name string) bool {
	newPath, err := fileinfo.CreateDirectoryPortable(fm.currentPath, name)
	if err != nil {
		debugPrint("FileManager: Create directory failed parent=%s name=%s err=%v", fm.currentPath, name, err)
		ui.ShowMessageDialog(fm.window, "Create directory failed", err.Error())
		return false
	}

	fm.applyCreatedDirectoryToList(newPath)
	debugPrint("FileManager: Created directory %s", newPath)
	fm.FocusFileList()
	return true
}

func (fm *FileManager) applyCreatedDirectoryToList(path string) {
	name := fileinfo.BaseName(path)
	info, err := fileinfo.StatPortable(path)
	if err != nil {
		debugPrint("FileManager: Created directory stat failed path=%s err=%v", path, err)
		fm.LoadDirectory(fm.currentPath)
		return
	}

	created := fileinfo.FileInfo{
		Name:     name,
		Path:     path,
		IsDir:    true,
		Size:     0,
		Modified: info.ModTime(),
		FileType: fileinfo.FileTypeDirectory,
		Status:   fileinfo.StatusNormal,
	}

	fm.mu.Lock()
	fm.originalFiles = upsertCreatedDirectory(fm.originalFiles, created)
	if fm.currentFilter != nil && fm.currentFilter.Pattern != "" {
		filtered, err := fileinfo.FilterFiles(fm.originalFiles, fm.currentFilter.Pattern)
		if err != nil {
			debugPrint("FileManager: Filter error after create: %v", err)
			fm.files = fm.originalFiles
		} else {
			fm.files = filtered
		}
	} else {
		fm.files = make([]fileinfo.FileInfo, len(fm.originalFiles))
		copy(fm.files, fm.originalFiles)
	}
	fm.sortFilesWithConfig(fm.CurrentSort())
	fm.cursorPath = path
	if fm.GetCurrentCursorIndex() < 0 && len(fm.files) > 0 {
		fm.SetCursorByIndex(0)
	}
	fm.rebuildFileBinding()
	fm.fileList.Refresh()
	fm.RefreshCursor()
	fm.updateStatusBar()
	fm.mu.Unlock()

	if fm.dirWatcher != nil {
		fm.dirWatcher.RefreshSnapshot()
	}
}

func upsertCreatedDirectory(files []fileinfo.FileInfo, created fileinfo.FileInfo) []fileinfo.FileInfo {
	for i := range files {
		if files[i].Path == created.Path {
			files[i] = created
			return files
		}
	}
	return append(files, created)
}
