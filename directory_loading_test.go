package main

import (
	"context"
	"errors"
	"testing"
)

func TestBeginDirectoryLoadCancelsPreviousLoad(t *testing.T) {
	fm := &FileManager{currentPath: t.TempDir()}

	firstCtx, firstID := fm.beginDirectoryLoad()
	secondCtx, secondID := fm.beginDirectoryLoad()

	if firstID == secondID {
		t.Fatal("load IDs should be unique")
	}
	if !errors.Is(firstCtx.Err(), context.Canceled) {
		t.Fatalf("first context error = %v, want context.Canceled", firstCtx.Err())
	}
	if !fm.directoryLoadActive(secondID) {
		t.Fatal("second load should be active")
	}

	fm.cancelDirectoryLoad(firstID)
	if !fm.directoryLoadActive(secondID) {
		t.Fatal("stale cancel should not cancel the active load")
	}

	fm.cancelDirectoryLoad(secondID)
	if !errors.Is(secondCtx.Err(), context.Canceled) {
		t.Fatalf("second context error = %v, want context.Canceled", secondCtx.Err())
	}
	if fm.directoryLoadActive(secondID) {
		t.Fatal("active load should be cleared after cancel")
	}
}

func TestFinishDirectoryLoadRejectsStaleLoad(t *testing.T) {
	fm := &FileManager{}

	_, firstID := fm.beginDirectoryLoad()
	_, secondID := fm.beginDirectoryLoad()

	if fm.finishDirectoryLoad(firstID) {
		t.Fatal("stale load should not finish")
	}
	if !fm.finishDirectoryLoad(secondID) {
		t.Fatal("active load should finish")
	}
	if fm.directoryLoadActive(secondID) {
		t.Fatal("active load should be cleared after finish")
	}
}
