package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
)

// trackNavigationHistoryJob applies the persisted-history effects after a job
// reaches a terminal state. The callback runs on the worker, while all state
// mutation is deliberately marshalled back onto Fyne's main goroutine, where
// the rest of runtime-state mutation happens.
func (fm *FileManager) trackNavigationHistoryJob(job *jobs.Job) {
	if fm == nil || job == nil {
		return
	}
	job.OnFinished(func(snapshot jobs.JobSnapshot) {
		fyne.Do(func() {
			fm.applyJobNavigationHistory(snapshot)
		})
	})
}

func (fm *FileManager) applyJobNavigationHistory(snapshot jobs.JobSnapshot) {
	if fm == nil || fm.state == nil || len(snapshot.Results) == 0 {
		return
	}

	changed := false
	notifyPath := ""
	for _, result := range snapshot.Results {
		switch snapshot.Type {
		case jobs.TypeCopy:
			if !result.SourceIsDir || result.Destination == "" {
				continue
			}
			fm.state.AddToNavigationHistory(canonicalNavigationHistoryPath(result.Destination), fm.navigationHistoryMaxEntries())
			changed = true
			notifyPath = result.Destination
		case jobs.TypeExtract:
			if !result.DestinationCreated || result.Destination == "" {
				continue
			}
			fm.state.AddToNavigationHistory(canonicalNavigationHistoryPath(result.Destination), fm.navigationHistoryMaxEntries())
			changed = true
			notifyPath = result.Destination
		case jobs.TypeDelete:
			if !result.SourceIsDir {
				continue
			}
			if removeNavigationHistoryTree(fm.state, result.Source) {
				changed = true
			}
		case jobs.TypeMove:
			if !result.SourceIsDir || result.Destination == "" {
				continue
			}
			if rewriteNavigationHistoryTree(fm.state, result.Source, result.Destination, fm.navigationHistoryMaxEntries()) {
				changed = true
				notifyPath = result.Destination
			}
		}
	}

	if changed {
		fm.saveNavigationHistoryMutation(notifyPath)
	}
}

func (fm *FileManager) updateNavigationHistoryAfterRename(oldPath, newPath string) {
	if fm == nil || fm.state == nil {
		return
	}
	if rewriteNavigationHistoryTree(fm.state, oldPath, newPath, fm.navigationHistoryMaxEntries()) {
		fm.saveNavigationHistoryMutation(newPath)
	}
}

func (fm *FileManager) navigationHistoryMaxEntries() int {
	if fm == nil || fm.config == nil {
		return 0
	}
	return fm.config.UI.NavigationHistory.MaxEntries
}

func (fm *FileManager) saveNavigationHistoryMutation(preferredPath string) {
	if fm == nil || fm.state == nil {
		return
	}
	if fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving navigation history: %v", err)
		}
	}
	fm.notifyNavigationHistoryChanged(canonicalNavigationHistoryPath(preferredPath))
}

// removeNavigationHistoryTree drops root and every path below it from both
// frecency history and saved (pinned) history.
func removeNavigationHistoryTree(state *config.State, root string) bool {
	if state == nil {
		return false
	}
	root = canonicalNavigationHistoryPath(root)
	if root == "" {
		return false
	}
	ensureNavigationHistoryStorage(state)

	changed := false
	entries := make([]string, 0, len(state.NavigationHistory.Entries))
	for _, entry := range state.NavigationHistory.Entries {
		if navigationHistoryPathWithin(entry, root) {
			delete(state.NavigationHistory.LastUsed, entry)
			delete(state.NavigationHistory.UseCount, entry)
			changed = true
			continue
		}
		entries = append(entries, entry)
	}
	if changed {
		state.NavigationHistory.Entries = entries
	}

	pinned := make([]string, 0, len(state.NavigationHistory.Pinned))
	for _, entry := range state.NavigationHistory.Pinned {
		if navigationHistoryPathWithin(entry, root) {
			changed = true
			continue
		}
		pinned = append(pinned, entry)
	}
	if len(pinned) != len(state.NavigationHistory.Pinned) {
		state.NavigationHistory.Pinned = pinned
	}
	return changed
}

// rewriteNavigationHistoryTree rebases root and its descendants after a
// directory rename or move. Pinned paths follow as well; otherwise the
// History dialog would retain stale locations. The rebased root is always
// registered as a new use, even when the old root was not already in history.
func rewriteNavigationHistoryTree(state *config.State, oldRoot, newRoot string, maxEntries int) bool {
	if state == nil {
		return false
	}
	oldRoot = canonicalNavigationHistoryPath(oldRoot)
	newRoot = canonicalNavigationHistoryPath(newRoot)
	if oldRoot == "" || newRoot == "" {
		return false
	}
	ensureNavigationHistoryStorage(state)

	entries := make([]string, 0, len(state.NavigationHistory.Entries))
	lastUsed := make(map[string]time.Time, len(state.NavigationHistory.LastUsed))
	useCount := make(map[string]int, len(state.NavigationHistory.UseCount))
	seen := make(map[string]bool, len(state.NavigationHistory.Entries))

	for _, entry := range state.NavigationHistory.Entries {
		canonical := canonicalNavigationHistoryPath(entry)
		updated, rebased := rebaseNavigationHistoryPath(canonical, oldRoot, newRoot)
		if rebased {
			canonical = updated
		}
		if canonical == "" {
			continue
		}

		last := state.NavigationHistory.LastUsed[entry]
		if previous, ok := state.NavigationHistory.LastUsed[canonical]; ok && previous.After(last) {
			last = previous
		}
		count := state.NavigationHistory.UseCount[entry]
		if count == 0 {
			count = state.NavigationHistory.UseCount[canonical]
		}
		if count <= 0 {
			count = 1
		}
		if seen[canonical] {
			if last.After(lastUsed[canonical]) {
				lastUsed[canonical] = last
			}
			useCount[canonical] += count
			continue
		}
		seen[canonical] = true
		entries = append(entries, canonical)
		lastUsed[canonical] = last
		useCount[canonical] = count
	}

	pinned := make([]string, 0, len(state.NavigationHistory.Pinned))
	pinnedSeen := make(map[string]bool, len(state.NavigationHistory.Pinned))
	for _, entry := range state.NavigationHistory.Pinned {
		canonical := canonicalNavigationHistoryPath(entry)
		updated, rebased := rebaseNavigationHistoryPath(canonical, oldRoot, newRoot)
		if rebased {
			canonical = updated
		}
		if canonical == "" || pinnedSeen[canonical] {
			continue
		}
		pinnedSeen[canonical] = true
		pinned = append(pinned, canonical)
	}

	state.NavigationHistory.Entries = entries
	state.NavigationHistory.LastUsed = lastUsed
	state.NavigationHistory.UseCount = useCount
	state.NavigationHistory.Pinned = pinned
	state.AddToNavigationHistory(newRoot, maxEntries)
	return true
}

func ensureNavigationHistoryStorage(state *config.State) {
	if state.NavigationHistory.Entries == nil {
		state.NavigationHistory.Entries = make([]string, 0)
	}
	if state.NavigationHistory.LastUsed == nil {
		state.NavigationHistory.LastUsed = make(map[string]time.Time)
	}
	if state.NavigationHistory.UseCount == nil {
		state.NavigationHistory.UseCount = make(map[string]int)
	}
	if state.NavigationHistory.Pinned == nil {
		state.NavigationHistory.Pinned = make([]string, 0)
	}
}

func navigationHistoryPathWithin(path, root string) bool {
	path = canonicalNavigationHistoryPath(path)
	root = canonicalNavigationHistoryPath(root)
	if path == "" || root == "" {
		return false
	}
	if path == root {
		return true
	}

	_, pathParsed, pathErr := fileinfo.CanonicalDisplayPath(path)
	_, rootParsed, rootErr := fileinfo.CanonicalDisplayPath(root)
	if pathErr != nil || rootErr != nil || pathParsed.Scheme != rootParsed.Scheme {
		return false
	}
	if pathParsed.Scheme == fileinfo.SchemeSMB {
		if !strings.EqualFold(pathParsed.Host, rootParsed.Host) || !strings.EqualFold(pathParsed.Share, rootParsed.Share) || len(pathParsed.Segments) < len(rootParsed.Segments) {
			return false
		}
		for i, segment := range rootParsed.Segments {
			if !strings.EqualFold(pathParsed.Segments[i], segment) {
				return false
			}
		}
		return true
	}
	if pathParsed.Scheme != fileinfo.SchemeFile {
		return false
	}
	pathForCompare, rootForCompare := path, root
	if runtime.GOOS == "windows" {
		pathForCompare = strings.ToLower(pathForCompare)
		rootForCompare = strings.ToLower(rootForCompare)
	}
	relative, err := filepath.Rel(rootForCompare, pathForCompare)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func rebaseNavigationHistoryPath(path, oldRoot, newRoot string) (string, bool) {
	if !navigationHistoryPathWithin(path, oldRoot) {
		return path, false
	}
	if path == oldRoot {
		return newRoot, true
	}

	_, pathParsed, pathErr := fileinfo.CanonicalDisplayPath(path)
	_, oldParsed, oldErr := fileinfo.CanonicalDisplayPath(oldRoot)
	_, newParsed, newErr := fileinfo.CanonicalDisplayPath(newRoot)
	if pathErr != nil || oldErr != nil || newErr != nil || pathParsed.Scheme != oldParsed.Scheme || oldParsed.Scheme != newParsed.Scheme {
		return path, false
	}
	if pathParsed.Scheme == fileinfo.SchemeSMB {
		suffix := pathParsed.Segments[len(oldParsed.Segments):]
		if len(suffix) == 0 {
			return newRoot, true
		}
		return strings.TrimRight(newRoot, "/") + "/" + strings.Join(suffix, "/"), true
	}
	if pathParsed.Scheme != fileinfo.SchemeFile {
		return path, false
	}
	relative, err := filepath.Rel(oldRoot, path)
	if err != nil || relative == "." {
		return newRoot, err == nil
	}
	return filepath.Join(newRoot, relative), true
}
