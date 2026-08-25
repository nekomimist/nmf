package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"nmf/internal/fileinfo"
)

func TestShortcutOpenGenerationRejectsCanceledAndStaleResults(t *testing.T) {
	fm := &FileManager{}
	first, firstCtx := fm.beginShortcutOpen()
	second, secondCtx := fm.beginShortcutOpen()
	if firstCtx.Err() == nil {
		t.Fatal("starting a replacement shortcut open should cancel the previous context")
	}
	if secondCtx.Err() != nil {
		t.Fatalf("replacement shortcut context error = %v, want active", secondCtx.Err())
	}

	if fm.invalidateShortcutOpen(first) {
		t.Fatal("stale shortcut cancellation should not cancel the active request")
	}
	if fm.finishShortcutOpen(first) {
		t.Fatal("stale shortcut result should not finish")
	}
	if !fm.invalidateShortcutOpen(second) {
		t.Fatal("active shortcut cancellation should succeed")
	}
	if secondCtx.Err() == nil {
		t.Fatal("active shortcut cancellation should cancel its context")
	}
	if fm.finishShortcutOpen(second) {
		t.Fatal("canceled shortcut result should not finish")
	}
}

func TestRunShortcutOpenNavigatesWithoutDefaultDelegation(t *testing.T) {
	openCalls := 0
	result := runShortcutOpen(
		context.Background(),
		`C:\links\docs.lnk`,
		func(context.Context, string) (string, bool, error) {
			return `smb://server/share/docs`, true, nil
		},
		func(string) error {
			openCalls++
			return nil
		},
	)

	if !result.navigate || result.directory != `smb://server/share/docs` {
		t.Fatalf("shortcut result = %+v, want directory navigation", result)
	}
	if result.delegated || openCalls != 0 {
		t.Fatalf("default delegation = %t calls=%d, want none", result.delegated, openCalls)
	}
}

func TestRunShortcutOpenTargetFailureDoesNotDelegate(t *testing.T) {
	targetErr := &fileinfo.ShortcutNavigationError{
		Stage:  fileinfo.ShortcutNavigationTarget,
		Path:   `C:\links\offline.lnk`,
		Target: `\\offline\share`,
		Err:    errors.New("network path unavailable"),
	}
	openCalls := 0
	result := runShortcutOpen(
		context.Background(),
		targetErr.Path,
		func(context.Context, string) (string, bool, error) {
			return "", false, targetErr
		},
		func(string) error {
			openCalls++
			return nil
		},
	)

	if result.navigate || result.delegated || !errors.Is(result.resolveErr, targetErr.Err) {
		t.Fatalf("shortcut result = %+v, want surfaced target failure", result)
	}
	if openCalls != 0 {
		t.Fatalf("default opener calls = %d, want zero for inaccessible target", openCalls)
	}
}

func TestRunShortcutOpenReadFailureDelegatesOnWorker(t *testing.T) {
	readErr := &fileinfo.ShortcutNavigationError{
		Stage: fileinfo.ShortcutNavigationRead,
		Path:  `C:\links\unknown.lnk`,
		Err:   errors.New("unsupported shortcut"),
	}
	opened := ""
	result := runShortcutOpen(
		context.Background(),
		readErr.Path,
		func(context.Context, string) (string, bool, error) {
			return "", false, readErr
		},
		func(path string) error {
			opened = path
			return nil
		},
	)

	if result.navigate || !result.delegated || result.resolveErr != readErr {
		t.Fatalf("shortcut result = %+v, want default delegation", result)
	}
	if opened != readErr.Path {
		t.Fatalf("default opener path = %q, want %q", opened, readErr.Path)
	}
}

func TestRunShortcutOpenCancellationSkipsDefaultDelegation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	resolverStarted := make(chan struct{})
	resultCh := make(chan shortcutOpenResult, 1)
	openCalls := 0
	go func() {
		resultCh <- runShortcutOpen(
			ctx,
			`C:\links\offline.lnk`,
			func(ctx context.Context, _ string) (string, bool, error) {
				close(resolverStarted)
				<-ctx.Done()
				return "", false, &fileinfo.ShortcutNavigationError{
					Stage: fileinfo.ShortcutNavigationRead,
					Err:   errors.New("late read failure"),
				}
			},
			func(string) error {
				openCalls++
				return nil
			},
		)
	}()

	<-resolverStarted
	cancel()
	result := <-resultCh
	if !errors.Is(result.resolveErr, context.Canceled) {
		t.Fatalf("shortcut result error = %v, want context.Canceled", result.resolveErr)
	}
	if result.delegated || openCalls != 0 {
		t.Fatalf("canceled default delegation = %t calls=%d, want none", result.delegated, openCalls)
	}
}

func TestOpenFileStartsShortcutResolutionWithoutBlocking(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	resolverReturned := make(chan struct{})
	fm := &FileManager{
		shortcutCandidateFn: func(string) bool { return true },
		shortcutResolverFn: func(context.Context, string) (string, bool, error) {
			close(started)
			<-release
			close(resolverReturned)
			return "", false, errors.New("late result")
		},
		shortcutDefaultOpenFn: func(string) error { return nil },
	}

	returned := make(chan struct{})
	go func() {
		fm.OpenFile(&fileinfo.FileInfo{Name: "offline.lnk", Path: `C:\links\offline.lnk`})
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("OpenFile blocked on shortcut resolution")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("shortcut resolver did not start")
	}
	if !fm.invalidateShortcutOpen(0) {
		close(release)
		t.Fatal("active shortcut operation was not cancellable")
	}
	close(release)
	select {
	case <-resolverReturned:
	case <-time.After(time.Second):
		t.Fatal("blocked test resolver did not return")
	}
}
