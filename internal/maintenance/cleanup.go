package maintenance

import (
	"fmt"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

type Task string

const (
	TaskCursorMemory      Task = "cursorMemory"
	TaskNavigationHistory Task = "navigationHistory"
)

type Options struct {
	CleanCursorMemory      bool
	CleanNavigationHistory bool
	SkipNetworkPaths       bool
	SkipRemovablePaths     bool
}

type PathClass struct {
	Network   bool
	Removable bool
}

type Candidate struct {
	Task   Task
	Path   string
	Reason string
}

type Result struct {
	ScannedCursorMemory      int
	ScannedNavigationHistory int
	SkippedNetwork           int
	SkippedRemovable         int
	Candidates               []Candidate
}

type ClassifyFunc func(path string) (PathClass, error)
type AccessibleFunc func(path string) error

func DefaultOptions() Options {
	return Options{
		CleanCursorMemory:      true,
		CleanNavigationHistory: true,
		SkipNetworkPaths:       true,
		SkipRemovablePaths:     true,
	}
}

func DefaultClassify(path string) (PathClass, error) {
	class, err := fileinfo.ClassifyPath(path)
	return PathClass{Network: class.Network, Removable: class.Removable}, err
}

func DefaultAccessible(path string) error {
	_, _, err := fileinfo.ResolveAccessibleDirectoryPath(path)
	return err
}

func Plan(cfg *config.Config, options Options, classify ClassifyFunc, accessible AccessibleFunc) Result {
	if classify == nil {
		classify = DefaultClassify
	}
	if accessible == nil {
		accessible = DefaultAccessible
	}

	var result Result
	if cfg == nil {
		return result
	}

	if options.CleanCursorMemory {
		for path := range cfg.UI.CursorMemory.Entries {
			result.ScannedCursorMemory++
			result.inspect(TaskCursorMemory, path, options, classify, accessible)
		}
	}

	if options.CleanNavigationHistory {
		for _, path := range cfg.UI.NavigationHistory.Entries {
			result.ScannedNavigationHistory++
			result.inspect(TaskNavigationHistory, path, options, classify, accessible)
		}
	}

	return result
}

func (r *Result) inspect(task Task, path string, options Options, classify ClassifyFunc, accessible AccessibleFunc) {
	class, err := classify(path)
	if err == nil {
		if options.SkipNetworkPaths && class.Network {
			r.SkippedNetwork++
			return
		}
		if options.SkipRemovablePaths && class.Removable {
			r.SkippedRemovable++
			return
		}
	}

	if err := accessible(path); err != nil {
		r.Candidates = append(r.Candidates, Candidate{
			Task:   task,
			Path:   path,
			Reason: fmt.Sprintf("%v", err),
		})
	}
}

func Apply(cfg *config.Config, result Result) int {
	if cfg == nil {
		return 0
	}

	removedCursor := make(map[string]bool)
	removedHistory := make(map[string]bool)
	for _, candidate := range result.Candidates {
		switch candidate.Task {
		case TaskCursorMemory:
			removedCursor[candidate.Path] = true
		case TaskNavigationHistory:
			removedHistory[candidate.Path] = true
		}
	}

	removed := 0
	for path := range removedCursor {
		if _, exists := cfg.UI.CursorMemory.Entries[path]; exists {
			delete(cfg.UI.CursorMemory.Entries, path)
			delete(cfg.UI.CursorMemory.LastUsed, path)
			removed++
		}
	}

	if len(removedHistory) > 0 {
		entries := cfg.UI.NavigationHistory.Entries[:0]
		for _, path := range cfg.UI.NavigationHistory.Entries {
			if removedHistory[path] {
				delete(cfg.UI.NavigationHistory.LastUsed, path)
				removed++
				continue
			}
			entries = append(entries, path)
		}
		cfg.UI.NavigationHistory.Entries = entries
	}

	return removed
}
