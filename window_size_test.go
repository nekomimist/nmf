package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestResetWindowSizeUsesInitialWindowSize(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := &FileManager{
		window:            app.NewWindow("test"),
		initialWindowSize: fyne.NewSize(900, 650),
	}
	fm.window.Resize(fyne.NewSize(1200, 800))

	fm.ResetWindowSize()

	got := fm.window.Canvas().Size()
	if got != fm.initialWindowSize {
		t.Fatalf("window size = %v, want %v", got, fm.initialWindowSize)
	}
}

func TestResetAllWindowSizesUsesRegisteredFileManagers(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resetFileManagerWindowTestRegistry(t)

	left := &FileManager{
		window:            app.NewWindow("left"),
		initialWindowSize: fyne.NewSize(800, 600),
	}
	right := &FileManager{
		window:            app.NewWindow("right"),
		initialWindowSize: fyne.NewSize(1000, 720),
	}
	left.window.Resize(fyne.NewSize(1200, 800))
	right.window.Resize(fyne.NewSize(640, 480))
	registerFileManagerWindow(left)
	registerFileManagerWindow(right)

	left.ResetAllWindowSizes()

	if got := left.window.Canvas().Size(); got != left.initialWindowSize {
		t.Fatalf("left window size = %v, want %v", got, left.initialWindowSize)
	}
	if got := right.window.Canvas().Size(); got != right.initialWindowSize {
		t.Fatalf("right window size = %v, want %v", got, right.initialWindowSize)
	}
}
