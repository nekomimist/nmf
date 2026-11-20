package main

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"

	"nmf/internal/filecompare"
	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// ShowCompareDialog opens the direct-directory comparison dialog.
func (fm *FileManager) ShowCompareDialog() {
	if fm.browserModel().Filter() != nil {
		fm.ClearFilter()
	}
	sourcePath := fm.GetCurrentPath()
	sourceEntries := fm.browserModel().SourceFiles()
	fileCount, directoryCount := 0, 0
	for _, entry := range sourceEntries {
		if !fileinfo.IsFileOperationTarget(entry) {
			continue
		}
		if entry.IsDir {
			directoryCount++
		} else {
			fileCount++
		}
	}

	dest := fm.buildDestinationCandidates()
	dlg := ui.NewCompareDialog(sourcePath, fileCount, directoryCount, dest, fm.keyManager, debugPrint, fm.searchMatchers)
	openDest := destinationCandidateOpenMap(dest)
	dlg.SetOnSelectedPathChanged(func(path string) {
		updateOpenPathHighlights(fm, path, openDest, dlg.SetOwnerHighlighted)
	})
	dlg.ShowDialog(fm.window, func(result ui.CompareResult) {
		fm.runCompare(sourcePath, sourceEntries, result)
	})
}

func (fm *FileManager) runCompare(sourcePath string, sourceEntries []fileinfo.FileInfo, result ui.CompareResult) {
	if result.Destination == "" {
		fm.FocusFileList()
		return
	}
	if sameDirectoryPath(result.Destination, sourcePath) {
		fm.ShowMessageDialog("Compare Directories", "Choose a different destination directory.")
		fm.FocusFileList()
		return
	}

	fm.cancelCompare()
	ctx, cancel := context.WithCancel(context.Background())
	fm.compareCancel = cancel
	fm.compareGeneration++
	generation := fm.compareGeneration
	fm.busy.Begin(fmt.Sprintf("Comparing %s...", result.Destination), func() {
		fm.cancelCompare()
		fm.FocusFileList()
	})
	go func() {
		compareResult, err := filecompare.CompareDirectEntriesContext(ctx, sourceEntries, result.Destination, result.Method)
		fyne.Do(func() {
			if fm.isWindowClosed() || generation != fm.compareGeneration || fm.compareCancel == nil {
				return
			}
			fm.cancelCompare()
			if err != nil {
				debugPrint("FileManager: Compare failed source=%s dest=%s method=%s err=%v", sourcePath, result.Destination, result.Method, err)
				fm.ShowMessageDialog("Compare Directories", err.Error())
				fm.FocusFileList()
				return
			}
			marked := fm.applyCompareMarks(compareResult.Matched)
			debugPrint("FileManager: Compare done source=%s dest=%s method=%s sourceEntries=%d targetEntries=%d matched=%d errors=%d",
				sourcePath, result.Destination, result.Method, compareResult.SourceCount, compareResult.TargetCount, marked, compareResult.ErrorCount)
			entryKind := "file(s)"
			if result.Method == filecompare.MissingSubdirectory {
				entryKind = "subdirectory entries"
			}
			message := fmt.Sprintf("Marked %d %s.", marked, entryKind)
			if compareResult.ErrorCount > 0 {
				message += fmt.Sprintf(" %d %s could not be compared: %v", compareResult.ErrorCount, entryKind, compareResult.FirstError)
			}
			fm.ShowMessageDialog("Compare Directories", message)
			fm.FocusFileList()
		})
	}()
}

func (fm *FileManager) applyCompareMarks(matched []fileinfo.FileInfo) int {
	fm.browserModel().ReplaceSelection(matched)
	if fm.fileList != nil {
		fm.fileList.Refresh()
	}
	fm.updateStatusBar()
	return len(matched)
}

// cancelCompare invalidates queued completions before releasing busy input.
// It is called only on the UI thread, including navigation and window close.
func (fm *FileManager) cancelCompare() {
	if fm.compareCancel == nil {
		return
	}
	fm.compareGeneration++
	fm.compareCancel()
	fm.compareCancel = nil
	if fm.busy != nil {
		fm.busy.End()
	}
}
