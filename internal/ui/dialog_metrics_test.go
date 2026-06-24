package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestResponsiveDialogWidthKeepsMinimumForSmallParent(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	w.Resize(fyne.NewSize(500, 400))

	got := responsiveDialogWidth(w, 600)

	if got != 600 {
		t.Fatalf("responsiveDialogWidth() = %v, want minimum 600", got)
	}
}

func TestResponsiveDialogWidthUsesParentRatio(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	w.Resize(fyne.NewSize(1200, 800))

	got := responsiveDialogWidth(w, 600)

	if got != 1080 {
		t.Fatalf("responsiveDialogWidth() = %v, want 1080", got)
	}
}

func TestResponsiveDialogWidthCapsMaximum(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	w.Resize(fyne.NewSize(2000, 1000))

	got := responsiveDialogWidthWithRatio(w, 640, renameDialogWidthRatio, renameDialogMaxWidth)

	if got != renameDialogMaxWidth {
		t.Fatalf("rename responsive width = %v, want cap %v", got, renameDialogMaxWidth)
	}
}

func TestLineEditDialogWidthUsesResponsiveOptionOnlyWhenEnabled(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	w.Resize(fyne.NewSize(1200, 800))

	fixed := NewLineEditDialog(LineEditDialogOptions{Width: 640}, nil)
	if got := fixed.dialogWidth(w); got != 640 {
		t.Fatalf("fixed line edit width = %v, want 640", got)
	}

	responsive := NewLineEditDialog(LineEditDialogOptions{
		Width:           640,
		ResponsiveWidth: true,
		WidthRatio:      renameDialogWidthRatio,
		MaxWidth:        renameDialogMaxWidth,
	}, nil)
	if got := responsive.dialogWidth(w); got != 840 {
		t.Fatalf("responsive line edit width = %v, want 840", got)
	}
}
