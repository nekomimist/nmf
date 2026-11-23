package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestModalDialogLifecycleAndCallbacks(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	w.Resize(fyne.NewSize(800, 600))

	closed := 0
	d := newModalDialog("Settings", widget.NewLabel("body"), w)
	d.SetOnClosed(func() { closed++ })

	d.Show()
	if got := len(w.Canvas().Overlays().List()); got != 1 {
		t.Fatalf("overlay count after Show = %d, want 1", got)
	}

	d.Hide()
	if got := len(w.Canvas().Overlays().List()); got != 0 {
		t.Fatalf("overlay count after Hide = %d, want 0", got)
	}
	if closed != 1 {
		t.Fatalf("onClosed count = %d, want 1", closed)
	}
}

func TestModalDialogOmitsHeaderWhenTitleAndTrailingAreEmpty(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()

	d := newModalDialog("", widget.NewLabel("viewer"), w).(*modalDialog)
	if d.shell.titleText != nil {
		t.Fatal("header title text should be omitted when title is empty")
	}
	if d.shell.headerDivider != nil {
		t.Fatal("header divider should be omitted when title is empty")
	}
}

func TestModalDialogAttachesDialogHighlightFrameToOuterShell(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()

	var highlight dialogHighlightState
	highlight.setHighlighted(true)

	d := newModalDialogWithOptions("History", widget.NewLabel("body"), w, modalDialogOptions{
		highlight: &highlight,
	}).(*modalDialog)
	d.Show()
	defer d.Hide()

	if highlight.frame == nil || !highlight.frame.IsHighlighted() {
		t.Fatal("dialog highlight frame should stay highlighted when attached to modal shell")
	}
	if got := highlight.frame.border.StrokeWidth; got != dialogHighlightFrameStrokeWidth {
		t.Fatalf("dialog highlight stroke width = %v, want %v", got, dialogHighlightFrameStrokeWidth)
	}
	if got := highlight.frame.border.CornerRadius; got <= 0 {
		t.Fatalf("dialog highlight corner radius = %v, want > 0", got)
	}
}

func TestInsetListSurfaceUsesInputSurfaceAndBorderColors(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	surface := newInsetListSurface(widget.NewLabel("item")).(*insetSurface)
	w := test.NewWindow(surface)
	defer w.Close()

	renderer := test.WidgetRenderer(surface).(*insetSurfaceRenderer)
	renderer.Refresh()

	if surface.background.StrokeWidth != insetSurfaceBorderWidth {
		t.Fatalf("stroke width = %v, want %v", surface.background.StrokeWidth, insetSurfaceBorderWidth)
	}
	wantFill := color.NRGBAModel.Convert(surface.Theme().Color(theme.ColorNameInputBackground, app.Settings().ThemeVariant()))
	if got := color.NRGBAModel.Convert(surface.background.FillColor); got != wantFill {
		t.Fatalf("fill color = %#v, want %#v", got, wantFill)
	}
}
