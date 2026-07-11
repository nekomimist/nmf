package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	"nmf/internal/ui"
)

// onJobsUpdated updates the Jobs indicator based on queue state
func (fm *FileManager) onJobsUpdated() {
	if fm.isWindowClosed() {
		return
	}
	mgr := fm.jobManager()
	snaps := mgr.List()
	var hasError, hasPending, hasRunning bool
	remainingJobs := 0
	for _, s := range snaps {
		switch s.Status {
		case jobs.StatusFailed:
			if !s.FailureAcknowledged {
				hasError = true
			}
		case jobs.StatusPending:
			hasPending = true
			remainingJobs++
		case jobs.StatusRunning:
			hasRunning = true
			remainingJobs++
		}
	}

	if fm.jobsButton == nil {
		return
	}
	fm.jobsButton.SetText(jobsButtonText(remainingJobs))

	// Visual policy:
	// - Error or Pending: blink
	// - Running only: highlight but no blink
	if hasError || hasPending {
		fm.jobsButton.Importance = widget.HighImportance
		if !fm.jobsBlinking {
			fm.startJobsBlink()
		}
	} else {
		if fm.jobsBlinking {
			fm.stopJobsBlink()
		}
		if hasRunning {
			fm.jobsButton.Importance = widget.HighImportance
		} else {
			fm.jobsButton.Importance = widget.MediumImportance
		}
	}
	fm.jobsButton.Refresh()
}

func jobsButtonText(remainingJobs int) string {
	if remainingJobs <= 0 {
		return "Jobs"
	}
	return fmt.Sprintf("Jobs (%d)", remainingJobs)
}

func (fm *FileManager) startJobsBlink() {
	if fm.jobsBlinking {
		return
	}
	fm.jobsBlinking = true
	fm.jobsBlinkStop = make(chan struct{})
	go func(stop <-chan struct{}) {
		ticker := time.NewTicker(600 * time.Millisecond)
		defer ticker.Stop()
		blinkOn := true
		for {
			select {
			case <-ticker.C:
				blinkOn = !blinkOn
				importanceOn := blinkOn
				// Toggle importance to create a blink effect
				fyne.Do(func() {
					select {
					case <-stop:
						return
					default:
					}
					if fm.isWindowClosed() {
						return
					}
					if fm.jobsButton != nil {
						if importanceOn {
							fm.jobsButton.Importance = widget.HighImportance
						} else {
							fm.jobsButton.Importance = widget.MediumImportance
						}
						fm.jobsButton.Refresh()
					}
				})
			case <-stop:
				return
			}
		}
	}(fm.jobsBlinkStop)
}

func (fm *FileManager) stopJobsBlink() {
	if !fm.jobsBlinking {
		return
	}
	close(fm.jobsBlinkStop)
	fm.jobsBlinking = false
}

// ShowCopyDialog shows the copy UI (simulation only)
func (fm *FileManager) ShowCopyDialog() { fm.showCopyMoveDialog(ui.OpCopy) }

// ShowMoveDialog shows the move UI (simulation only)
func (fm *FileManager) ShowMoveDialog() { fm.showCopyMoveDialog(ui.OpMove) }

// ShowExtractArchiveDialog shows the archive extraction UI.
func (fm *FileManager) ShowExtractArchiveDialog() {
	targets, srcPaths := fm.collectArchiveTargets()
	if len(targets) == 0 {
		debugPrint("FileManager: No archive target for extract")
		fm.ShowMessageDialog("Extract failed", "Select a supported archive file to extract.")
		return
	}
	fm.showTransferDestinationDialog(ui.OpExtract, targets, func(result ui.CopyMoveResult) {
		fm.jobManager().EnqueueExtractWithOptions(srcPaths, result.Destination, fm.conflictResolver(), jobs.TransferOptions{PreserveTimestamps: result.PreserveTimestamps})
		fm.FocusFileList()
	})
}

// showCopyMoveDialog builds targets and destination candidates then shows dialog
func (fm *FileManager) showCopyMoveDialog(op ui.Operation) {
	// Determine targets: marked files if any; otherwise cursor item
	targets := fm.collectTargets()
	if len(targets) == 0 {
		debugPrint("FileManager: No valid target for %s", string(op))
		return
	}

	// We need full source paths for jobs, not only names — compute now
	srcPaths := fm.collectTargetPaths()
	fm.showTransferDestinationDialog(op, targets, func(result ui.CopyMoveResult) {
		selectedDest := result.Destination
		if op == ui.OpMove && sameDirectoryPath(selectedDest, fm.currentPath) {
			debugPrint("FileManager: %s destination is current directory; no-op dest=%s", strings.Title(string(op)), selectedDest)
			fm.FocusFileList()
			return
		}

		mgr := fm.jobManager()
		resolver := fm.conflictResolver()
		if op == ui.OpCopy {
			mgr.EnqueueCopyWithOptions(srcPaths, selectedDest, resolver, jobs.TransferOptions{PreserveTimestamps: result.PreserveTimestamps})
		} else {
			mgr.EnqueueMoveWithResolver(srcPaths, selectedDest, resolver)
		}
		fm.FocusFileList()
	})
}

func (fm *FileManager) showTransferDestinationDialog(op ui.Operation, targets []string, onAccept func(ui.CopyMoveResult)) {
	dest := fm.buildDestinationCandidates()
	if len(dest) == 0 {
		debugPrint("FileManager: No destination candidates available")
	}
	dlg := ui.NewCopyMoveDialog(op, targets, dest, fm.state.NavigationHistory.LastUsed, fm.config.UI.Copy.PreserveTimestamps, fm.keyManager, debugPrint, fm.searchMatchers)
	openDest := destinationCandidateOpenMap(dest)
	refreshDestinations := func(preferredPath string) {
		dest = fm.buildDestinationCandidates()
		openDest = destinationCandidateOpenMap(dest)
		dlg.SetDestinations(dest, preferredPath)
	}
	unsubscribe := subscribeNavigationHistoryChanged(func(path string) {
		fyne.Do(func() {
			refreshDestinations(path)
		})
	})
	dlg.SetOnSelectedPathChanged(func(path string) {
		if openDest[path] {
			highlightFileManagerWindowForPath(path)
			return
		}
		clearFileManagerWindowHighlights()
	})
	dlg.SetOnOpenDestination(func(path string) {
		fm.openWindowAtPath(path)
		refreshDestinations(path)
	})
	dlg.SetOnClosed(func() {
		clearFileManagerWindowHighlights()
		unsubscribe()
	})
	dlg.ShowDialog(fm.window, onAccept)
}

func (fm *FileManager) conflictResolver() jobs.ConflictResolver {
	if fm.runtime == nil || fm.runtime.promptBroker == nil {
		return nil
	}
	return fm.runtime.promptBroker.ResolveConflict
}

func sameDirectoryPath(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}

	resolvedA, _, errA := fileinfo.ResolvePathDisplay(a)
	if errA == nil {
		a = resolvedA
	}
	resolvedB, _, errB := fileinfo.ResolvePathDisplay(b)
	if errB == nil {
		b = resolvedB
	}

	return normalizeComparablePath(a) == normalizeComparablePath(b)
}

func normalizeComparablePath(p string) string {
	p = strings.TrimSpace(p)
	if fileinfo.IsSMBDisplay(p) {
		p = strings.ReplaceAll(p, "\\", "/")
		p = strings.TrimRight(p, "/")
		return strings.ToLower(p)
	}

	cleaned := filepath.Clean(p)
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	return cleaned
}

// collectTargets returns display names of targets based on selection or cursor
func (fm *FileManager) collectTargets() []string {
	selectedFiles := fm.selectedFileInfos()
	selected := make([]string, len(selectedFiles))
	for i, fi := range selectedFiles {
		selected[i] = fi.Name
	}
	if len(selected) > 0 {
		return selected
	}
	// Fall back to cursor
	idx := fm.GetCurrentCursorIndex()
	if idx >= 0 && idx < len(fm.files) {
		fi := fm.files[idx]
		if fi.Name != ".." && fi.Status != fileinfo.StatusDeleted {
			return []string{fi.Name}
		}
	}
	return nil
}

// collectTargetPaths returns absolute/native source file paths
func (fm *FileManager) collectTargetPaths() []string {
	selectedFiles := fm.selectedFileInfos()
	selected := make([]string, len(selectedFiles))
	for i, fi := range selectedFiles {
		selected[i] = fi.Path
	}
	if len(selected) > 0 {
		return selected
	}
	idx := fm.GetCurrentCursorIndex()
	if idx >= 0 && idx < len(fm.files) {
		fi := fm.files[idx]
		if fi.Name != ".." && fi.Status != fileinfo.StatusDeleted {
			return []string{fi.Path}
		}
	}
	return nil
}

func (fm *FileManager) collectArchiveTargets() ([]string, []string) {
	selectedFiles := fm.selectedFileInfos()
	if len(selectedFiles) > 0 {
		return archiveTargetNamesAndPaths(selectedFiles)
	}
	idx := fm.GetCurrentCursorIndex()
	if idx >= 0 && idx < len(fm.files) {
		fi := fm.files[idx]
		if isTargetFileInfo(fi) {
			return archiveTargetNamesAndPaths([]fileinfo.FileInfo{fi})
		}
	}
	return nil, nil
}

func archiveTargetNamesAndPaths(files []fileinfo.FileInfo) ([]string, []string) {
	names := make([]string, 0, len(files))
	paths := make([]string, 0, len(files))
	for _, fi := range files {
		if fi.IsDir || fileinfo.IsArchivePath(fi.Path) || !fileinfo.IsSupportedArchive(fi.Path) {
			continue
		}
		names = append(names, fi.Name)
		paths = append(paths, fi.Path)
	}
	return names, paths
}

// ShowJobsDialog opens the job queue view
func (fm *FileManager) ShowJobsDialog() {
	if fm.runtime != nil && fm.runtime.jobsWindowController != nil {
		fm.runtime.jobsWindowController.Show()
	}
}

// buildDestinationCandidates composes other windows' dirs then history without dups.
func (fm *FileManager) buildDestinationCandidates() []ui.DestinationCandidate {
	// Collect from other windows
	seen := map[string]int{}
	var candidates []ui.DestinationCandidate
	windowRegistry.Range(func(k, v any) bool {
		if other, ok := v.(*FileManager); ok {
			if other == fm {
				return true
			}
			if other.currentPath != "" {
				if fileinfo.IsArchivePath(other.currentPath) {
					return true
				}
				if idx, ok := seen[other.currentPath]; ok {
					candidates[idx].OpenInOtherWindow = true
				} else {
					seen[other.currentPath] = len(candidates)
					candidates = append(candidates, ui.DestinationCandidate{
						Path:              other.currentPath,
						OpenInOtherWindow: true,
					})
				}
			}
		}
		return true
	})

	// Optionally include current path after other windows
	if fm.currentPath != "" && !fileinfo.IsArchivePath(fm.currentPath) {
		if _, ok := seen[fm.currentPath]; !ok {
			seen[fm.currentPath] = len(candidates)
			candidates = append(candidates, ui.DestinationCandidate{Path: fm.currentPath})
		}
	}

	// Append navigation history skipping dups
	for _, p := range fm.state.GetNavigationHistory() {
		if fileinfo.IsArchivePath(p) {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = len(candidates)
		candidates = append(candidates, ui.DestinationCandidate{Path: p})
	}
	return candidates
}

func destinationCandidateOpenMap(candidates []ui.DestinationCandidate) map[string]bool {
	result := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if candidate.OpenInOtherWindow {
			result[candidate.Path] = true
		}
	}
	return result
}
