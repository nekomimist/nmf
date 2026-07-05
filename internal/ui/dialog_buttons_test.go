package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestDialogButtonRowUsesCancelAndPrimaryConfirmButtons(t *testing.T) {
	row := dialogButtonRow("Cancel", func() {}, "OK", func() {})
	cancel, confirm := rowButtons(t, row)

	if cancel.Text != "Cancel" {
		t.Fatalf("cancel text = %q, want Cancel", cancel.Text)
	}
	if cancel.Icon == nil || cancel.Icon.Name() != theme.CancelIcon().Name() {
		t.Fatalf("cancel icon = %v, want cancel icon", cancel.Icon)
	}
	if cancel.Importance == widget.HighImportance {
		t.Fatal("cancel button should not be high importance")
	}

	if confirm.Text != "OK" {
		t.Fatalf("confirm text = %q, want OK", confirm.Text)
	}
	if confirm.Icon == nil || confirm.Icon.Name() != theme.ConfirmIcon().Name() {
		t.Fatalf("confirm icon = %v, want confirm icon", confirm.Icon)
	}
	if confirm.Importance != widget.HighImportance {
		t.Fatalf("confirm importance = %v, want high", confirm.Importance)
	}
}

func TestDialogOKButtonRowUsesPrimaryConfirmButton(t *testing.T) {
	row := dialogOKButtonRow(func() {})
	buttons := barButtons(t, row)
	if len(buttons) != 1 {
		t.Fatalf("button count = %d, want 1", len(buttons))
	}
	button := buttons[0]
	if button.Text != "OK" {
		t.Fatalf("button text = %q, want OK", button.Text)
	}
	if button.Icon == nil || button.Icon.Name() != theme.ConfirmIcon().Name() {
		t.Fatalf("button icon = %v, want confirm icon", button.Icon)
	}
	if button.Importance != widget.HighImportance {
		t.Fatalf("button importance = %v, want high", button.Importance)
	}
}

func TestDialogButtonBarAppliesMinWidthFloor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	button := widget.NewButtonWithIcon("OK", theme.ConfirmIcon(), func() {})
	bar := dialogButtonBar(button)
	wrappers := barWrappers(t, bar)
	if len(wrappers) != 1 {
		t.Fatalf("wrapper count = %d, want 1", len(wrappers))
	}

	rawWidth := button.MinSize().Width
	gotWidth := wrappers[0].MinSize().Width
	if gotWidth != dialogButtonMinWidth {
		t.Fatalf("wrapper MinSize().Width = %v, want %v", gotWidth, dialogButtonMinWidth)
	}
	if gotWidth < rawWidth {
		t.Fatalf("wrapper MinSize().Width = %v, want >= raw button width %v", gotWidth, rawWidth)
	}
}

func TestDialogButtonBarDoesNotCapLongLabels(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	button := widget.NewButtonWithIcon("This Is A Deliberately Long Label", theme.ConfirmIcon(), func() {})
	bar := dialogButtonBar(button)
	wrappers := barWrappers(t, bar)
	if len(wrappers) != 1 {
		t.Fatalf("wrapper count = %d, want 1", len(wrappers))
	}

	rawWidth := button.MinSize().Width
	if rawWidth <= dialogButtonMinWidth {
		t.Fatalf("raw button width = %v, want > floor %v for this test to be meaningful", rawWidth, dialogButtonMinWidth)
	}
	if got := wrappers[0].MinSize().Width; got != rawWidth {
		t.Fatalf("wrapper MinSize().Width = %v, want unchanged raw width %v", got, rawWidth)
	}
}

func TestDialogButtonBarPreservesOrder(t *testing.T) {
	first := widget.NewButton("First", func() {})
	second := widget.NewButton("Second", func() {})
	third := widget.NewButton("Third", func() {})

	bar := dialogButtonBar(first, second, third)
	buttons := barButtons(t, bar)
	if len(buttons) != 3 {
		t.Fatalf("button count = %d, want 3", len(buttons))
	}
	want := []string{"First", "Second", "Third"}
	for i, b := range buttons {
		if b.Text != want[i] {
			t.Fatalf("button[%d] text = %q, want %q", i, b.Text, want[i])
		}
	}
}

// barWrappers peels a dialogButtonBar's Center -> HBox nesting and returns
// the per-button minWidth wrapper containers, in bar order.
func barWrappers(t *testing.T, bar fyne.CanvasObject) []*fyne.Container {
	t.Helper()
	center, ok := bar.(*fyne.Container)
	if !ok {
		t.Fatalf("bar type = %T, want *fyne.Container", bar)
	}
	if len(center.Objects) != 1 {
		t.Fatalf("center object count = %d, want 1", len(center.Objects))
	}
	hbox, ok := center.Objects[0].(*fyne.Container)
	if !ok {
		t.Fatalf("hbox type = %T, want *fyne.Container", center.Objects[0])
	}
	wrappers := make([]*fyne.Container, len(hbox.Objects))
	for i, obj := range hbox.Objects {
		wrapper, ok := obj.(*fyne.Container)
		if !ok {
			t.Fatalf("wrapper %d type = %T, want *fyne.Container", i, obj)
		}
		wrappers[i] = wrapper
	}
	return wrappers
}

// barButtons peels wrapper containers down to their wrapped buttons, in
// bar order.
func barButtons(t *testing.T, bar fyne.CanvasObject) []*widget.Button {
	t.Helper()
	wrappers := barWrappers(t, bar)
	buttons := make([]*widget.Button, len(wrappers))
	for i, wrapper := range wrappers {
		if len(wrapper.Objects) != 1 {
			t.Fatalf("wrapper %d object count = %d, want 1", i, len(wrapper.Objects))
		}
		button, ok := wrapper.Objects[0].(*widget.Button)
		if !ok {
			t.Fatalf("wrapper %d child type = %T, want *widget.Button", i, wrapper.Objects[0])
		}
		buttons[i] = button
	}
	return buttons
}

// rowButtons is a convenience wrapper for the common two-button case (used
// by dialog_buttons_test.go and quit_dialog_test.go).
func rowButtons(t *testing.T, row fyne.CanvasObject) (*widget.Button, *widget.Button) {
	t.Helper()
	buttons := barButtons(t, row)
	if len(buttons) != 2 {
		t.Fatalf("button count = %d, want 2", len(buttons))
	}
	return buttons[0], buttons[1]
}
