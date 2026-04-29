package main

import (
	"context"
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
	mgr := jobs.GetManager()
	snaps := mgr.List()
	var hasError, hasPending, hasRunning bool
	for _, s := range snaps {
		switch s.Status {
		case jobs.StatusFailed:
			hasError = true
		case jobs.StatusPending:
			hasPending = true
		case jobs.StatusRunning:
			hasRunning = true
		}
	}

	if fm.jobsButton == nil {
		return
	}

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

func (fm *FileManager) startJobsBlink() {
	if fm.jobsBlinking {
		return
	}
	fm.jobsBlinking = true
	fm.jobsBlinkOn = true
	fm.jobsBlinkStop = make(chan struct{})
	go func(stop <-chan struct{}) {
		ticker := time.NewTicker(600 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fm.jobsBlinkOn = !fm.jobsBlinkOn
				// Toggle importance to create a blink effect
				fyne.Do(func() {
					if fm.jobsButton != nil {
						if fm.jobsBlinkOn {
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
	fm.jobsBlinkOn = false
}

// ShowCopyDialog shows the copy UI (simulation only)
func (fm *FileManager) ShowCopyDialog() { fm.showCopyMoveDialog(ui.OpCopy) }

// ShowMoveDialog shows the move UI (simulation only)
func (fm *FileManager) ShowMoveDialog() { fm.showCopyMoveDialog(ui.OpMove) }

// showCopyMoveDialog builds targets and destination candidates then shows dialog
func (fm *FileManager) showCopyMoveDialog(op ui.Operation) {
	// Determine targets: marked files if any; otherwise cursor item
	targets := fm.collectTargets()
	if len(targets) == 0 {
		debugPrint("FileManager: No valid target for %s", string(op))
		return
	}

	// Build destination candidates: other windows' directories first, then history without duplicates
	dest := fm.buildDestinationCandidates()
	if len(dest) == 0 {
		debugPrint("FileManager: No destination candidates available")
	}

	// We need full source paths for jobs, not only names — compute now
	srcPaths := fm.collectTargetPaths()
	dlg := ui.NewCopyMoveDialog(op, targets, dest, fm.config.UI.NavigationHistory.LastUsed, fm.keyManager, debugPrint)
	dlg.ShowDialog(fm.window, func(selectedDest string) {
		if op == ui.OpMove && sameDirectoryPath(selectedDest, fm.currentPath) {
			debugPrint("FileManager: %s destination is current directory; no-op dest=%s", strings.Title(string(op)), selectedDest)
			fm.FocusFileList()
			return
		}

		mgr := jobs.GetManager()
		resolver := fm.conflictResolver()
		if op == ui.OpCopy {
			mgr.EnqueueCopyWithResolver(srcPaths, selectedDest, resolver)
		} else {
			mgr.EnqueueMoveWithResolver(srcPaths, selectedDest, resolver)
		}
		// Feedback
		ui.ShowMessageDialog(fm.window, strings.Title(string(op)), fmt.Sprintf("Queued %d item(s) to:\n%s", len(srcPaths), selectedDest))
		fm.FocusFileList()
	})
}

func (fm *FileManager) conflictResolver() jobs.ConflictResolver {
	return func(ctx context.Context, req jobs.ConflictRequest) jobs.ConflictResolution {
		done := make(chan jobs.ConflictResolution, 1)
		fyne.Do(func() {
			if fm.window == nil {
				done <- jobs.ConflictResolution{Action: jobs.ConflictCancelJob}
				return
			}
			dlg := ui.NewConflictDialog(req, fm.keyManager)
			dlg.ShowDialog(fm.window, func(res jobs.ConflictResolution) {
				done <- res
			})
		})

		select {
		case <-ctx.Done():
			return jobs.ConflictResolution{Action: jobs.ConflictCancelJob}
		case res := <-done:
			return res
		}
	}
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
	// Gather selected files
	var selected []string
	for p, sel := range fm.selectedFiles {
		if !sel {
			continue
		}
		// Find matching file to ensure it still exists in list and to skip parent/invalid
		for _, fi := range fm.files {
			if fi.Path == p {
				if fi.Name == ".." || fi.Status == fileinfo.StatusDeleted {
					continue
				}
				selected = append(selected, fi.Name)
				break
			}
		}
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
	var selected []string
	for p, sel := range fm.selectedFiles {
		if !sel {
			continue
		}
		for _, fi := range fm.files {
			if fi.Path == p {
				if fi.Name == ".." || fi.Status == fileinfo.StatusDeleted {
					continue
				}
				selected = append(selected, fi.Path)
				break
			}
		}
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

// ShowJobsDialog opens the job queue view
func (fm *FileManager) ShowJobsDialog() {
	showJobsWindow()
}

// buildDestinationCandidates composes other windows' dirs then history without dups
func (fm *FileManager) buildDestinationCandidates() []string {
	// Collect from other windows
	seen := map[string]struct{}{}
	var candidates []string
	windowRegistry.Range(func(k, v any) bool {
		if other, ok := v.(*FileManager); ok {
			if other == fm {
				return true
			}
			if other.currentPath != "" {
				if _, ok := seen[other.currentPath]; !ok {
					candidates = append(candidates, other.currentPath)
					seen[other.currentPath] = struct{}{}
				}
			}
		}
		return true
	})

	// Optionally include current path after other windows
	if fm.currentPath != "" {
		if _, ok := seen[fm.currentPath]; !ok {
			candidates = append(candidates, fm.currentPath)
			seen[fm.currentPath] = struct{}{}
		}
	}

	// Append navigation history skipping dups
	for _, p := range fm.config.GetNavigationHistory() {
		if _, ok := seen[p]; ok {
			continue
		}
		candidates = append(candidates, p)
		seen[p] = struct{}{}
	}
	return candidates
}
