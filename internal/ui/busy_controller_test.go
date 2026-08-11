package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"nmf/internal/keymanager"
)

func TestBusyControllerOwnsOneInputGuard(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	controller := NewBusyController(nil, km, busyOverlayTheme{}, time.Hour, nil)
	cancelled := 0
	controller.Begin("Loading first...", func() { cancelled++ })

	if !controller.Active() {
		t.Fatal("controller should be active after Begin")
	}
	if got := km.GetStackSize(); got != 1 {
		t.Fatalf("key handler stack = %d, want one busy guard", got)
	}
	if controller.overlay.IsVisible() {
		t.Fatal("delayed overlay became visible immediately")
	}

	controller.Begin("Loading parent...", nil)
	if got := km.GetStackSize(); got != 1 {
		t.Fatalf("key handler stack after update = %d, want the same busy guard", got)
	}
	if !controller.overlay.IsVisible() {
		t.Fatal("active Begin should update and reveal the existing overlay")
	}
	if got := controller.overlay.label.text.Text; got != "Loading parent..." {
		t.Fatalf("overlay text = %q, want updated text", got)
	}

	handler := km.GetCurrentHandler()
	if handler == nil || !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, keymanager.ModifierState{}) {
		t.Fatal("busy guard did not handle Escape")
	}
	if cancelled != 1 {
		t.Fatalf("cancel callback count = %d, want 1", cancelled)
	}

	controller.End()
	if controller.Active() || controller.overlay.IsVisible() {
		t.Fatal("End left busy state visible or active")
	}
	if got := km.GetStackSize(); got != 0 {
		t.Fatalf("key handler stack after End = %d, want empty", got)
	}
	controller.End()
	if got := km.GetStackSize(); got != 0 {
		t.Fatalf("repeated End changed key handler stack to %d", got)
	}
}

func TestBusyControllerRejectsStaleDelayedShow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	controller := NewBusyController(nil, nil, busyOverlayTheme{}, time.Hour, nil)
	controller.Begin("Loading...", nil)
	generation := controller.generation
	controller.End()

	controller.showIfCurrent(generation)
	if controller.overlay.IsVisible() {
		t.Fatal("stale delayed callback revived an ended overlay")
	}
	controller.UpdateText("stale")
	if controller.overlay.IsVisible() {
		t.Fatal("UpdateText revived an inactive overlay")
	}
}

func TestBusyControllerShowsAfterConfiguredDelay(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	scheduled := make(chan func(), 1)
	controller := NewBusyController(nil, nil, busyOverlayTheme{}, 5*time.Millisecond, nil)
	controller.runOnMain = func(fn func()) {
		scheduled <- fn
	}
	controller.Begin("Loading...", nil)
	defer controller.End()

	select {
	case show := <-scheduled:
		if controller.overlay.IsVisible() {
			t.Fatal("overlay became visible before scheduled UI work ran")
		}
		show()
		if !controller.overlay.IsVisible() {
			t.Fatal("scheduled UI work did not reveal the overlay")
		}
	case <-time.After(time.Second):
		t.Fatal("delayed overlay did not become visible")
	}
}
