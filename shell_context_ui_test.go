package main

import (
	"testing"

	"fyne.io/fyne/v2/canvas"
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
