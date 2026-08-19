package main

import (
	"strings"
	"unicode/utf8"

	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// ShowRenameDialog shows a direct single-item rename dialog.
func (fm *FileManager) ShowRenameDialog() {
	idx := fm.GetCurrentCursorIndex()
	target, ok := fm.FileAt(idx)
	if !ok {
		debugPrint("FileManager: No valid target for rename")
		return
	}

	if target.Name == ".." || target.Status == fileinfo.StatusDeleted {
		debugPrint("FileManager: Invalid rename target: %s", target.Name)
		return
	}

	dlg := ui.NewLineEditDialog(ui.LineEditDialogOptions{
		Title:            "Rename",
		Prompt:           "New name:",
		CurrentText:      target.Name,
		InitialText:      target.Name,
		InitialSelection: renameInitialSelection(target),
		ConfirmText:      "Rename",
		ResponsiveWidth:  true,
		WidthRatio:       ui.RenameDialogWidthRatio(),
		MaxWidth:         ui.RenameDialogMaxWidth(),
	}, fm.keyManager, fm.config.UI.KeyBindings)
	dlg.ShowDialog(fm.window, func(newName string) bool {
		return fm.renameCurrentFile(target, newName)
	})
}

func renameInitialSelection(target fileinfo.FileInfo) *ui.LineEditSelection {
	name := target.Name
	dot := strings.LastIndex(name, ".")
	if target.IsDir || dot <= 0 || dot == len(name)-1 {
		return &ui.LineEditSelection{
			Start: 0,
			End:   utf8.RuneCountInString(name),
		}
	}
	return &ui.LineEditSelection{
		Start: 0,
		End:   utf8.RuneCountInString(name[:dot]),
	}
}

func (fm *FileManager) renameCurrentFile(target fileinfo.FileInfo, newName string) bool {
	if fm.directoryListingNavigationOnly() {
		fm.logCachedListingBlocked("rename")
		return false
	}
	trimmed := strings.TrimSpace(newName)
	if trimmed == target.Name {
		fm.FocusFileList()
		return true
	}

	newPath, err := fileinfo.RenamePortable(target.Path, trimmed)
	if err != nil {
		debugPrint("FileManager: Rename failed %s -> %s: %v", target.Path, trimmed, err)
		fm.ShowMessageDialog("Rename failed", err.Error())
		return false
	}

	fm.applyRenameToList(target.Path, trimmed, newPath)
	if target.IsDir {
		fm.updateNavigationHistoryAfterRename(target.Path, newPath)
	}
	debugPrint("FileManager: Renamed %s -> %s", target.Path, newPath)
	fm.FocusFileList()
	return true
}

func (fm *FileManager) applyRenameToList(oldPath, newName, newPath string) {
	if !fm.browserModel().Rename(oldPath, newName, newPath) {
		return
	}

	fm.RefreshCursor()

	if fm.dirWatcher != nil {
		fm.dirWatcher.RefreshSnapshot()
	}
}
