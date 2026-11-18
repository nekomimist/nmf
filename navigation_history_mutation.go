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

	var rules historyPathRules
	var destinations []string
	for i, result := range snapshot.Results {
		if snapshot.Type == jobs.TypeExtract {
			if !result.DestinationCreated {
				continue
			}
		} else if !result.SourceIsDir {
			continue
		}
		switch snapshot.Type {
		case jobs.TypeDelete:
			rules.add(i, result.Source, "")
		case jobs.TypeMove, jobs.TypeCopy, jobs.TypeExtract:
			dst := canonicalHistoryMutationPath(result.Destination)
			if dst == "" {
				continue
			}
			if snapshot.Type == jobs.TypeMove {
				if result.Source == "" {
					continue
				}
				rules.add(len(destinations), result.Source, dst)
			}
			destinations = append(destinations, dst)
		}
	}

	changed := rules.apply(fm.state)
	preferred := ""
	if len(destinations) != 0 {
		preferred = destinations[len(destinations)-1]
		if snapshot.Type == jobs.TypeMove {
			// A later result can move an earlier destination again. Each
			// visit belongs to the final location of that directory.
			for i, dst := range destinations {
				destinations[i] = rules.rewrite(dst, i+1)
			}
		}
		fm.state.AddNavigationHistoryPaths(destinations, fm.navigationHistoryMaxEntries())
		changed = true
	}
	if changed {
		fm.saveNavigationHistoryMutation(preferred)
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
		if err := fm.scheduleStateSave(); err != nil {
			debugPrint("FileManager: Error saving navigation history: %v", err)
		}
	}
	fm.notifyNavigationHistoryChanged(canonicalNavigationHistoryPath(preferredPath))
}

type historyPathRule struct {
	order       int
	destination string // Empty for deletion.
}

// Index the old roots so a history entry only checks its own ancestors.
// Rules are ordered to preserve chained and nested moves within a job.
type historyPathRules map[string][]historyPathRule

func (r *historyPathRules) add(order int, oldPath, destination string) {
	src := canonicalHistoryMutationPath(oldPath)
	if src == "" {
		return
	}
	if *r == nil {
		*r = make(historyPathRules)
	}
	key := historyRootKey(src)
	(*r)[key] = append((*r)[key], historyPathRule{order, destination})
}

func canonicalHistoryMutationPath(path string) string {
	display, parsed, err := fileinfo.CanonicalDisplayPath(path)
	if err != nil {
		return path
	}
	if parsed.Scheme == fileinfo.SchemeFile {
		return filepath.Clean(display)
	}
	return display
}

func historyRootKey(path string) string {
	if runtime.GOOS == "windows" || fileinfo.IsSMBDisplay(path) {
		return strings.ToLower(path)
	}
	return path
}

func (rules historyPathRules) rewrite(path string, start int) string {
	// Directory jobs operate on the archive's backing path. An explicitly
	// encoded archive address need not share its backing directory's prefix.
	if outer, inner, ok := fileinfo.SplitArchivePath(path); ok {
		updated := rules.rewrite(outer, start)
		if updated == "" {
			return ""
		}
		if updated == outer {
			return path
		}
		return fileinfo.ArchiveDisplayPath(updated, inner)
	}
	for {
		var rule *historyPathRule
		var ancestor string
		for probe := path; ; {
			candidates := rules[historyRootKey(probe)]
			for i := range candidates {
				candidate := &candidates[i]
				if candidate.order >= start {
					if rule == nil || candidate.order < rule.order {
						rule, ancestor = candidate, probe
					}
					break
				}
			}
			parent := fileinfo.ParentPath(probe)
			if parent == probe || len(parent) >= len(probe) {
				break
			}
			probe = parent
		}
		if rule == nil {
			return path
		}
		if rule.destination == "" {
			return ""
		}
		// The suffix retains the source spelling, even on case-insensitive
		// providers. Join using the destination's separator and namespace.
		suffix := strings.TrimPrefix(filepath.ToSlash(path[len(ancestor):]), "/")
		path = rule.destination
		if suffix != "" {
			path = fileinfo.JoinPath(path, suffix)
		}
		start = rule.order + 1
	}
}

func (rules historyPathRules) apply(state *config.State) bool {
	if len(rules) == 0 {
		return false
	}
	history := &state.NavigationHistory
	remapped := make(map[string]string)
	for _, paths := range [][]string{history.Entries, history.Pinned} {
		for _, entry := range paths {
			if _, found := remapped[entry]; found {
				continue
			}
			if dst := rules.rewrite(canonicalHistoryMutationPath(entry), 0); dst != entry {
				remapped[entry] = dst
			}
		}
	}
	if len(remapped) == 0 {
		return false
	}

	updated := config.NavigationHistoryState{
		Entries:  make([]string, 0, len(history.Entries)),
		Pinned:   make([]string, 0, len(history.Pinned)),
		LastUsed: make(map[string]time.Time, len(history.LastUsed)),
		UseCount: make(map[string]int, len(history.UseCount)),
	}
	for _, entry := range history.Entries {
		dst, changed := remapped[entry]
		if !changed {
			dst = entry
		}
		if dst == "" {
			continue
		}
		last, count := navigationHistoryUsage(*history, entry, dst)
		if _, exists := updated.UseCount[dst]; !exists {
			updated.Entries = append(updated.Entries, dst)
		}
		updated.UseCount[dst] += count
		if prev, exists := updated.LastUsed[dst]; !exists || last.After(prev) {
			updated.LastUsed[dst] = last
		}
	}
	seenPins := make(map[string]bool, len(history.Pinned))
	for _, pin := range history.Pinned {
		dst, changed := remapped[pin]
		if !changed {
			dst = pin
		}
		if dst != "" && !seenPins[dst] {
			seenPins[dst] = true
			updated.Pinned = append(updated.Pinned, dst)
		}
	}
	*history = updated
	return true
}

func rewriteNavigationHistoryTree(state *config.State, oldPath, newPath string, maxEntries int) bool {
	dst := canonicalNavigationHistoryPath(newPath)
	if state == nil || dst == "" || oldPath == "" {
		return false
	}
	var rules historyPathRules
	rules.add(0, oldPath, dst)
	rules.apply(state)
	state.AddToNavigationHistory(dst, maxEntries)
	return true
}

// navigationHistoryUsage preserves legacy entries whose statistics were stored
// under the canonical path, and gives entries without a positive count one use.
func navigationHistoryUsage(history config.NavigationHistoryState, entry, canonical string) (time.Time, int) {
	last := history.LastUsed[entry]
	if previous, ok := history.LastUsed[canonical]; ok && previous.After(last) {
		last = previous
	}
	count := history.UseCount[entry]
	if count == 0 {
		count = history.UseCount[canonical]
	}
	if count <= 0 {
		count = 1
	}
	return last, count
}
