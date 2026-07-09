package main

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestJobsWindowControllerShowReusesWindow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	c := NewJobsWindowController(app, debugPrint)

	c.Show()
	first := c.window
	if first == nil {
		t.Fatal("Show should create a Jobs window")
	}

	c.Show()
	if c.window != first {
		t.Fatal("Show should reuse the existing Jobs window")
	}

	c.Close()
}

func TestJobsWindowControllerCloseClearsWindow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	c := NewJobsWindowController(app, debugPrint)

	c.Show()
	c.Close()

	if c.window != nil {
		t.Fatal("Close should clear the Jobs window")
	}
}

func TestJobsWindowControllerShowRecreatesAfterUserClose(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	c := NewJobsWindowController(app, debugPrint)

	c.Show()
	first := c.window
	first.Window().Close() // simulate the user closing the window directly

	c.Show()
	if c.window == nil {
		t.Fatal("Show should recreate the Jobs window after it was closed")
	}
	if c.window == first {
		t.Fatal("Show should not reuse a window that was already closed")
	}

	c.Close()
}
