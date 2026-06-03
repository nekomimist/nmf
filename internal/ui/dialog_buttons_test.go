package ui

import (
	"testing"

	"fyne.io/fyne/v2"
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
	container, ok := row.(*fyne.Container)
	if !ok {
		t.Fatalf("row type = %T, want *fyne.Container", row)
	}
	if len(container.Objects) != 1 {
		t.Fatalf("button count = %d, want 1", len(container.Objects))
	}
	button, ok := container.Objects[0].(*widget.Button)
	if !ok {
		t.Fatalf("button type = %T, want *widget.Button", container.Objects[0])
	}
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

func rowButtons(t *testing.T, row interface{}) (*widget.Button, *widget.Button) {
	t.Helper()
	container, ok := row.(*fyne.Container)
	if !ok {
		t.Fatalf("row type = %T, want *fyne.Container", row)
	}
	if len(container.Objects) != 2 {
		t.Fatalf("button count = %d, want 2", len(container.Objects))
	}
	cancel, ok := container.Objects[0].(*widget.Button)
	if !ok {
		t.Fatalf("cancel type = %T, want *widget.Button", container.Objects[0])
	}
	confirm, ok := container.Objects[1].(*widget.Button)
	if !ok {
		t.Fatalf("confirm type = %T, want *widget.Button", container.Objects[1])
	}
	return cancel, confirm
}
