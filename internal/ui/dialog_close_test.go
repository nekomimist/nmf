package ui

import (
	"testing"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestUnfocusIfDialogOwnedClearsOwnedFocus(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := app.NewWindow("test")
	owned := widget.NewEntry()
	other := widget.NewEntry()
	w.SetContent(container.NewVBox(owned, other))
	w.Canvas().Focus(owned)

	unfocusIfDialogOwned(w, owned)

	if got := w.Canvas().Focused(); got != nil {
		t.Fatalf("focused = %T, want nil", got)
	}
}

func TestUnfocusIfDialogOwnedPreservesExternalFocus(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := app.NewWindow("test")
	owned := widget.NewEntry()
	other := widget.NewEntry()
	w.SetContent(container.NewVBox(owned, other))
	w.Canvas().Focus(other)

	unfocusIfDialogOwned(w, owned)

	if got := w.Canvas().Focused(); got != other {
		t.Fatalf("focused = %T, want external entry", got)
	}
}
