package ui

import (
	"image/color"
	"testing"

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
