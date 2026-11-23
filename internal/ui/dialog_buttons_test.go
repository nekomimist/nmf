package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestDialogButtonRowUsesCancelAndPrimaryConfirmButtons(t *testing.T) {
	row := dialogButtonRow("Cancel", func() {}, "OK", func() {})
	cancel, confirm := rowButtons(t, row)

	if cancel.Text != "Cancel" {
		t.Fatalf("cancel text = %q, want Cancel", cancel.Text)
	}
	if cancel.Icon != nil {
		t.Fatalf("cancel icon = %v, want nil", cancel.Icon)
	}
	if cancel.Importance == widget.HighImportance {
		t.Fatal("cancel button should not be high importance")
	}

	if confirm.Text != "OK" {
		t.Fatalf("confirm text = %q, want OK", confirm.Text)
	}
	if confirm.Icon != nil {
		t.Fatalf("confirm icon = %v, want nil", confirm.Icon)
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
	if button.Icon != nil {
		t.Fatalf("button icon = %v, want nil", button.Icon)
	}
	if button.Importance != widget.HighImportance {
		t.Fatalf("button importance = %v, want high", button.Importance)
	}
}

func TestDialogButtonBarRightAlignsWithLeadingSpacer(t *testing.T) {
	bar := dialogButtonBar(dialogCancelButton("Cancel", func() {}), dialogConfirmButton("OK", func() {}))
	hbox := barHBox(t, bar)
	if len(hbox.Objects) != 3 {
		t.Fatalf("hbox object count = %d, want 3 (spacer + 2 buttons)", len(hbox.Objects))
	}
	if _, ok := hbox.Objects[0].(*layout.Spacer); !ok {
		t.Fatalf("first object type = %T, want *layout.Spacer for right alignment", hbox.Objects[0])
	}
}

func TestDialogSplitButtonBarPlacesSpacerBetweenLeadingAndTrailing(t *testing.T) {
	bar := dialogSplitButtonBar(
		[]*widget.Button{dialogAuxButton("Scan", nil, func() {})},
		[]*widget.Button{dialogCancelButton("Close", func() {}), dialogConfirmButton("Apply", func() {})},
	)
	hbox := barHBox(t, bar)
	if len(hbox.Objects) != 4 {
		t.Fatalf("hbox object count = %d, want 4 (1 leading + spacer + 2 trailing)", len(hbox.Objects))
	}
	if _, ok := hbox.Objects[1].(*layout.Spacer); !ok {
		t.Fatalf("middle object type = %T, want *layout.Spacer", hbox.Objects[1])
	}
	buttons := barButtons(t, bar)
	want := []string{"Scan", "Close", "Apply"}
	for i, b := range buttons {
		if b.Text != want[i] {
			t.Fatalf("button[%d] text = %q, want %q", i, b.Text, want[i])
		}
	}
}

func TestDialogButtonBarAppliesMinWidthFloor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	button := widget.NewButton("OK", func() {})
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

	button := widget.NewButton("This Is A Deliberately Long Label", func() {})
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

func barHBox(t *testing.T, bar fyne.CanvasObject) *fyne.Container {
	t.Helper()
	outer, ok := bar.(*fyne.Container)
	if !ok {
		t.Fatalf("bar type = %T, want *fyne.Container", bar)
	}
	if len(outer.Objects) != 1 {
		t.Fatalf("outer object count = %d, want 1", len(outer.Objects))
	}
	footer, ok := outer.Objects[0].(*fyne.Container)
	if !ok {
		t.Fatalf("footer type = %T, want *fyne.Container", outer.Objects[0])
	}
	if len(footer.Objects) != 2 {
		t.Fatalf("footer object count = %d, want 2 (separator + button row)", len(footer.Objects))
	}
	if _, ok := footer.Objects[0].(*widget.Separator); !ok {
		t.Fatalf("footer[0] type = %T, want *widget.Separator", footer.Objects[0])
	}
	rowPad, ok := footer.Objects[1].(*fyne.Container)
	if !ok || len(rowPad.Objects) != 1 {
		t.Fatalf("footer[1] = %T, want padded container with 1 child", footer.Objects[1])
	}
	hbox, ok := rowPad.Objects[0].(*fyne.Container)
	if !ok {
		t.Fatalf("hbox type = %T, want *fyne.Container", rowPad.Objects[0])
	}
	return hbox
}

// barWrappers peels a dialogButtonBar's padded HBox nesting and returns
// the per-button minWidth wrapper containers, in bar order.
func barWrappers(t *testing.T, bar fyne.CanvasObject) []*fyne.Container {
	t.Helper()
	hbox := barHBox(t, bar)
	var wrappers []*fyne.Container
	for _, obj := range hbox.Objects {
		if _, isSpacer := obj.(*layout.Spacer); isSpacer {
			continue
		}
		wrapper, ok := obj.(*fyne.Container)
		if !ok {
			t.Fatalf("wrapper type = %T, want *fyne.Container", obj)
		}
		wrappers = append(wrappers, wrapper)
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
