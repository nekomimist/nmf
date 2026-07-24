package maintenance

import (
	"context"
	"fmt"
	"sort"

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
	SkipUnavailableVolumes bool
}

type PathClass struct {
	Network     bool
	Removable   bool
	Unavailable bool
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
	SkippedUnavailable       int
	Candidates               []Candidate
}

// Targets is an immutable snapshot of the runtime-state paths inspected by a
// maintenance scan.
type Targets struct {
	CursorMemory      []string
	NavigationHistory []string
}

type ClassifyFunc func(path string) (PathClass, error)
type AccessibleFunc func(path string) error
type AccessibleContextFunc func(ctx context.Context, path string) error

func DefaultOptions() Options {
	return Options{
		CleanCursorMemory:      true,
		CleanNavigationHistory: true,
		SkipNetworkPaths:       true,
		SkipRemovablePaths:     true,
		SkipUnavailableVolumes: true,
	}
}

func DefaultClassify(path string) (PathClass, error) {
	class, err := fileinfo.ClassifyPath(path)
	return PathClass{
		Network:     class.Network,
		Removable:   class.Removable,
		Unavailable: class.Unavailable,
	}, err
}

func DefaultAccessible(path string) error {
	return DefaultAccessibleContext(context.Background(), path)
}

func DefaultAccessibleContext(ctx context.Context, path string) error {
	// Maintenance deliberately does not inspect archive contents. Opening an
	// encrypted archive can prompt for a password, and an unsupported or
	// damaged archive is not sufficient reason to discard its remembered
	// internal paths. Retain those paths whenever the backing file still
	// exists; if it has gone away, the normal Stat error makes them candidates.
	if archiveFile, _, ok := fileinfo.SplitArchivePath(path); ok {
		_, err := fileinfo.StatPortableContext(ctx, archiveFile)
		return err
	}
	_, _, err := fileinfo.ResolveAccessibleDirectoryPathContext(ctx, path)
	return err
}

// Snapshot copies maintenance targets so a background scan never iterates
// runtime-state maps while the UI may be changing them.
func Snapshot(state *config.State) Targets {
	var targets Targets
	if state == nil {
		return targets
	}

	targets.CursorMemory = make([]string, 0, len(state.CursorMemory.Entries))
	for path := range state.CursorMemory.Entries {
		targets.CursorMemory = append(targets.CursorMemory, path)
	}
	sort.Strings(targets.CursorMemory)
	targets.NavigationHistory = append(targets.NavigationHistory, state.NavigationHistory.Entries...)
	return targets
}

func Plan(state *config.State, options Options, classify ClassifyFunc, accessible AccessibleFunc) Result {
	targets := Snapshot(state)
	accessibleContext := func(_ context.Context, path string) error {
		return DefaultAccessible(path)
	}
	if accessible != nil {
		accessibleContext = func(_ context.Context, path string) error {
			return accessible(path)
		}
	}
	result, _ := PlanTargets(context.Background(), targets, options, classify, accessibleContext)
	return result
}

// PlanTargets inspects a stable target snapshot. Cancellation is checked
// between paths; callers can therefore abandon a background scan immediately
// even if the current platform call takes time to return.
func PlanTargets(ctx context.Context, targets Targets, options Options, classify ClassifyFunc, accessible AccessibleContextFunc) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if classify == nil {
		classify = DefaultClassify
	}
	if accessible == nil {
		accessible = DefaultAccessibleContext
	}

	var result Result
	if options.CleanCursorMemory {
		for _, path := range targets.CursorMemory {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			result.ScannedCursorMemory++
			result.inspect(ctx, TaskCursorMemory, path, options, classify, accessible)
		}
	}

	if options.CleanNavigationHistory {
		for _, path := range targets.NavigationHistory {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			result.ScannedNavigationHistory++
			result.inspect(ctx, TaskNavigationHistory, path, options, classify, accessible)
		}
	}

	return result, ctx.Err()
}

func (r *Result) inspect(ctx context.Context, task Task, path string, options Options, classify ClassifyFunc, accessible AccessibleContextFunc) {
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
		if options.SkipUnavailableVolumes && class.Unavailable {
			r.SkippedUnavailable++
			return
		}
	}

	if ctx.Err() != nil {
		return
	}
	if err := accessible(ctx, path); err != nil {
		r.Candidates = append(r.Candidates, Candidate{
			Task:   task,
			Path:   path,
			Reason: fmt.Sprintf("%v", err),
		})
	}
}

func Apply(state *config.State, result Result) int {
	if state == nil {
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
		if _, exists := state.CursorMemory.Entries[path]; exists {
			delete(state.CursorMemory.Entries, path)
			delete(state.CursorMemory.LastUsed, path)
			removed++
		}
	}

	if len(removedHistory) > 0 {
		entries := state.NavigationHistory.Entries[:0]
		for _, path := range state.NavigationHistory.Entries {
			if removedHistory[path] {
				delete(state.NavigationHistory.LastUsed, path)
				delete(state.NavigationHistory.UseCount, path)
				removed++
				continue
			}
			entries = append(entries, path)
		}
		state.NavigationHistory.Entries = entries
	}

	return removed
}
