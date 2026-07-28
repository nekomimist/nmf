package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	customtheme "nmf/internal/theme"
)

type highlightFrameTheme struct{}

func (highlightFrameTheme) GetCustomColor(name string) color.RGBA {
	if name == customtheme.ColorCopyMoveOpenDestination {
		return color.RGBA{R: 11, G: 22, B: 33, A: 244}
	}
	return color.RGBA{}
}

func TestHighlightFrameUsesConfiguredColorAndClears(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	frame := NewHighlightFrame(highlightFrameTheme{})
	if frame.IsHighlighted() {
		t.Fatal("new highlight frame should be inactive")
	}
	if got := color.RGBAModel.Convert(frame.border.StrokeColor).(color.RGBA); got.A != 0 {
		t.Fatalf("inactive stroke color = %#v, want transparent", got)
	}

	frame.SetHighlighted(true)

	if !frame.IsHighlighted() {
		t.Fatal("highlight frame should be active")
	}
	if got, want := color.RGBAModel.Convert(frame.border.StrokeColor).(color.RGBA), (color.RGBA{R: 11, G: 22, B: 33, A: 244}); got != want {
		t.Fatalf("active stroke color = %#v, want %#v", got, want)
	}
	if frame.border.StrokeWidth != highlightFrameStrokeWidth {
		t.Fatalf("stroke width = %v, want %v", frame.border.StrokeWidth, highlightFrameStrokeWidth)
	}

	frame.SetHighlighted(false)

	if got := color.RGBAModel.Convert(frame.border.StrokeColor).(color.RGBA); got.A != 0 {
		t.Fatalf("cleared stroke color = %#v, want transparent", got)
	}
}

func TestHighlightFrameWrapContentReservesFixedInset(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	frame := NewHighlightFrame(nil)
	content := widget.NewLabel("content")
	wrapped := frame.WrapContent(content)
	wrapped.Resize(fyne.NewSize(100, 80))

	if got, want := content.Position(), fyne.NewPos(highlightFrameContentInset, highlightFrameContentInset); got != want {
		t.Fatalf("content position = %v, want %v", got, want)
	}
	if got, want := content.Size(), fyne.NewSize(92, 72); got != want {
		t.Fatalf("content size = %v, want %v", got, want)
	}
	if got := frame.Position(); got != fyne.NewPos(0, 0) {
		t.Fatalf("frame position = %v, want origin", got)
	}
	if got, want := frame.Size(), fyne.NewSize(100, 80); got != want {
		t.Fatalf("frame size = %v, want %v", got, want)
	}
}

func TestDialogHighlightStateKeepsStateBeforeWrap(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	var state dialogHighlightState
	state.setHighlighted(true)
	wrapped := state.wrap(widget.NewLabel("content"))

	if wrapped == nil {
		t.Fatal("wrapped dialog content should not be nil")
	}
	if state.frame == nil || !state.frame.IsHighlighted() {
		t.Fatal("dialog frame should retain highlight set before wrap")
	}

	state.setHighlighted(false)
	if state.frame.IsHighlighted() {
		t.Fatal("dialog frame should clear after wrap")
	}
}
