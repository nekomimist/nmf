package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	fynetest "fyne.io/fyne/v2/test"

	"nmf/internal/fileinfo"
)

func TestShortcutOpenGenerationRejectsCanceledAndStaleResults(t *testing.T) {
	fm := &FileManager{}
	first, firstCtx := fm.beginFileOperation()
	second, secondCtx := fm.beginFileOperation()
	if firstCtx.Err() == nil {
		t.Fatal("starting a replacement shortcut open should cancel the previous context")
	}
	if secondCtx.Err() != nil {
		t.Fatalf("replacement shortcut context error = %v, want active", secondCtx.Err())
	}

	if fm.invalidateFileOperation(first) {
		t.Fatal("stale shortcut cancellation should not cancel the active request")
	}
	if fm.finishFileOperation(first) {
		t.Fatal("stale shortcut result should not finish")
	}
	if !fm.invalidateFileOperation(second) {
		t.Fatal("active shortcut cancellation should succeed")
	}
	if secondCtx.Err() == nil {
		t.Fatal("active shortcut cancellation should cancel its context")
	}
	if fm.finishFileOperation(second) {
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
	if !fm.invalidateFileOperation(0) {
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

func TestOpenOrdinaryFileWithArchiveSuffixUsesListedOSPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a temporary Unix opener")
	}
	app := fynetest.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	report := filepath.Join(dir, "opened-path")
	t.Setenv("NMF_TEST_OPENED_FILE", report)
	t.Setenv("PATH", dir)
	if err := os.WriteFile(filepath.Join(dir, "xdg-open"), []byte(`#!/bin/sh
printf %s "$1" > "$NMF_TEST_OPENED_FILE"
`), 0700); err != nil {
		t.Fatal(err)
	}
	name := "ordinary.zip\u2003"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("plain text"), 0600); err != nil {
		t.Fatal(err)
	}
	fm := &FileManager{}
	fm.OpenFile(&fileinfo.FileInfo{Path: path, Name: name})
	deadline := time.Now().Add(3 * time.Second)
	for {
		got, _ := os.ReadFile(report)
		fm.fileOperationMu.Lock()
		finished := fm.activeFileOperationID == 0
		fm.fileOperationMu.Unlock()
		if finished && string(got) == path {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("opener got %q, want %q (finished=%t)", got, path, finished)
		}
		time.Sleep(time.Millisecond)
	}
}
