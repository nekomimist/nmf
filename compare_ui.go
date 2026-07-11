package main

import (
	"fmt"

	"fyne.io/fyne/v2"

	"nmf/internal/filecompare"
	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// ShowCompareDialog opens the direct-directory comparison dialog.
func (fm *FileManager) ShowCompareDialog() {
	sourceFiles := fm.compareSourceFiles()
	if len(sourceFiles) == 0 {
		debugPrint("FileManager: No source files available for compare path=%s", fm.currentPath)
		fm.ShowMessageDialog("Compare Directories", "There are no files to compare in the current directory.")
		return
	}

	dest := fm.buildDestinationCandidates()
	dlg := ui.NewCompareDialog(fm.currentPath, len(sourceFiles), dest, fm.keyManager, debugPrint, fm.searchMatchers)
	openDest := destinationCandidateOpenMap(dest)
	dlg.SetOnSelectedPathChanged(func(path string) {
		if openDest[path] {
			highlightFileManagerWindowForPath(path)
			return
		}
		clearFileManagerWindowHighlights()
	})
	sourcePath := fm.currentPath
	dlg.ShowDialog(fm.window, func(result ui.CompareResult) {
		fm.runCompare(sourcePath, sourceFiles, result)
	})
}

func (fm *FileManager) compareSourceFiles() []fileinfo.FileInfo {
	if fm.currentFilter != nil {
		fm.ClearFilter()
	}

	fm.mu.RLock()
	defer fm.mu.RUnlock()

	source := fm.originalFiles
	if len(source) == 0 {
		source = fm.files
	}
	files := make([]fileinfo.FileInfo, 0, len(source))
	for _, fi := range source {
		if fi.Name == ".." || fi.IsDir || fi.Status == fileinfo.StatusDeleted {
			continue
		}
		files = append(files, fi)
	}
	return files
}

func (fm *FileManager) runCompare(sourcePath string, sourceFiles []fileinfo.FileInfo, result ui.CompareResult) {
	if result.Destination == "" {
		fm.FocusFileList()
		return
	}
	if sameDirectoryPath(result.Destination, sourcePath) {
		fm.ShowMessageDialog("Compare Directories", "Choose a different destination directory.")
		fm.FocusFileList()
		return
	}

	fm.beginBusy(fmt.Sprintf("Comparing %s...", result.Destination))
	go func() {
		compareResult, err := filecompare.CompareDirectFiles(sourceFiles, result.Destination, result.Method)
		fyne.Do(func() {
			if fm.isWindowClosed() {
				return
			}
			fm.endBusy()
			if err != nil {
				debugPrint("FileManager: Compare failed source=%s dest=%s method=%s err=%v", sourcePath, result.Destination, result.Method, err)
				fm.ShowMessageDialog("Compare Directories", err.Error())
				fm.FocusFileList()
				return
			}
			marked := fm.applyCompareMarks(compareResult.Matched)
			debugPrint("FileManager: Compare done source=%s dest=%s method=%s sourceFiles=%d targetFiles=%d matched=%d errors=%d",
				sourcePath, result.Destination, result.Method, compareResult.SourceCount, compareResult.TargetCount, marked, compareResult.ErrorCount)
			if compareResult.ErrorCount > 0 {
				fm.ShowMessageDialog("Compare Directories", fmt.Sprintf("Marked %d file(s). %d file(s) could not be compared: %v", marked, compareResult.ErrorCount, compareResult.FirstError))
			} else {
				fm.ShowMessageDialog("Compare Directories", fmt.Sprintf("Marked %d file(s).", marked))
			}
			fm.FocusFileList()
		})
	}()
}

func (fm *FileManager) applyCompareMarks(matched []fileinfo.FileInfo) int {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.selectedFiles = make(map[string]bool, len(matched))
	for _, fi := range matched {
		fm.selectedFiles[fi.Path] = true
	}
	if fm.fileList != nil {
		fm.fileList.Refresh()
	}
	fm.updateStatusBar()
	return len(matched)
}
