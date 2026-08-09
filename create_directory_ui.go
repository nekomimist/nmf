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
	}, fm.keyManager, fm.config.UI.KeyBindings)
	dlg.ShowDialog(fm.window, func(name string) bool {
		return fm.CreateDirectory(name)
	})
}

// CreateDirectory creates a directory under the current path and selects it.
func (fm *FileManager) CreateDirectory(name string) bool {
	currentPath := fm.GetCurrentPath()
	newPath, err := fileinfo.CreateDirectoryPortable(currentPath, name)
	if err != nil {
		debugPrint("FileManager: Create directory failed parent=%s name=%s err=%v", currentPath, name, err)
		fm.ShowMessageDialog("Create directory failed", err.Error())
		return false
	}

	fm.applyCreatedPathToList(newPath, true)
	fm.recordNavigationHistory(newPath)
	debugPrint("FileManager: Created directory %s", newPath)
	fm.FocusFileList()
	return true
}

func (fm *FileManager) applyCreatedPathToList(path string, isDir bool) {
	name := fileinfo.BaseName(path)
	info, err := fileinfo.StatPortable(path)
	if err != nil {
		debugPrint("FileManager: Created path stat failed path=%s err=%v", path, err)
		fm.LoadDirectory(fm.GetCurrentPath())
		return
	}

	created := fileinfo.FileInfo{
		Name:     name,
		Path:     path,
		IsDir:    isDir,
		Size:     info.Size(),
		Modified: info.ModTime(),
		FileType: fileinfo.DetermineFileType(path, name, isDir),
		Status:   fileinfo.StatusNormal,
	}
	if isDir {
		created.Size = 0
		created.FileType = fileinfo.FileTypeDirectory
	}

	if err := fm.browserModel().Upsert(created); err != nil {
		debugPrint("FileManager: Filter error after create: %v", err)
	}
	if fm.GetCurrentCursorIndex() < 0 && fm.FileCount() > 0 {
		fm.SetCursorByIndex(0)
	}
	if fm.fileList != nil {
		// RefreshCursor alone covers the redraw: it calls fileList.Refresh
		// directly when there's no cursor, or ScrollTo (which itself ends in
		// a full Refresh) when there is one.
		fm.RefreshCursor()
	}
	fm.updateStatusBar()

	if fm.dirWatcher != nil {
		fm.dirWatcher.RefreshSnapshot()
	}
}
