package main

import (
	"errors"
	"fmt"
	"testing"

	"fyne.io/fyne/v2/canvas"

	"nmf/internal/shellmenu"
)

func TestCursorMenuClientPositionWithoutAnchorFallsBack(t *testing.T) {
	fm := &FileManager{cursorPath: "/tmp/a"}

	_, _, ok := fm.cursorMenuClientPosition()

	if ok {
		t.Fatal("cursorMenuClientPosition ok = true, want false")
	}
}

func TestCursorMenuClientPositionWithStaleAnchorFallsBack(t *testing.T) {
	fm := &FileManager{
		cursorPath:   "/tmp/a",
		cursorAnchor: cursorRowAnchor{path: "/tmp/b", object: canvas.NewRectangle(nil)},
	}

	_, _, ok := fm.cursorMenuClientPosition()

	if ok {
		t.Fatal("cursorMenuClientPosition ok = true, want false")
	}
}

func TestExplorerContextMenuErrorMessage(t *testing.T) {
	t.Run("long path", func(t *testing.T) {
		message, ok := explorerContextMenuErrorMessage(errors.New("context: " + shellmenu.ErrPathTooLong.Error()))
		if ok {
			t.Fatal("plain error text should not be treated as ErrPathTooLong")
		}

		message, ok = explorerContextMenuErrorMessage(fmt.Errorf("context: %w", shellmenu.ErrPathTooLong))
		if !ok {
			t.Fatal("ErrPathTooLong should have a user-facing message")
		}
		want := "The Explorer Menu cannot be displayed because the file path is too long."
		if message != want {
			t.Fatalf("message = %q, want %q", message, want)
		}
	})

	if _, ok := explorerContextMenuErrorMessage(errors.New("other failure")); ok {
		t.Fatal("unrelated error should not have a specialized message")
	}
}
