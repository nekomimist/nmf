package ui

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func TestFileNameLabelMinSizeDoesNotUseFullNameWidth(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	label := NewFileNameLabel(strings.Repeat("a", 200)+".txt", color.RGBA{})

	if got := label.MinSize().Width; got != 0 {
		t.Fatalf("MinSize().Width = %v, want 0", got)
	}
}

func TestFileNameLabelDisplayTextFitsAssignedWidth(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	label := NewFileNameLabel(strings.Repeat("a", 80)+".txt", color.RGBA{})
	width := fyne.MeasureText("aaaaaaaa...aaaa.txt", label.text.TextSize, label.text.TextStyle).Width

	got := label.displayText(width)
	if !strings.Contains(got, "...") {
		t.Fatalf("displayText() = %q, want ellipsis", got)
	}
	if textWidth(got, label.text.TextSize, label.text.TextStyle) > width {
		t.Fatalf("displayText() width = %v, want <= %v", textWidth(got, label.text.TextSize, label.text.TextStyle), width)
	}
	if !strings.HasSuffix(got, ".txt") {
		t.Fatalf("displayText() = %q, want suffix preserved", got)
	}
}

func TestFileNameLabelDisplayTextReturnsEmptyForNoWidth(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	label := NewFileNameLabel("file.txt", color.RGBA{})

	if got := label.displayText(0); got != "" {
		t.Fatalf("displayText(0) = %q, want empty string", got)
	}
}

func TestTappableIconMouseMovedStartsDragAfterThreshold(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	calls := 0
	icon := NewTappableIcon(nil, nil)
	icon.SetOnDragged(func() { calls++ })

	icon.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{AbsolutePosition: fyne.NewPos(10, 10)},
		Button:     desktop.MouseButtonPrimary,
	})
	icon.MouseMoved(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{AbsolutePosition: fyne.NewPos(12, 12)},
		Button:     desktop.MouseButtonPrimary,
	})
	if calls != 0 {
		t.Fatalf("drag calls = %d, want 0 before threshold", calls)
	}

	icon.MouseMoved(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{AbsolutePosition: fyne.NewPos(13, 13)},
		Button:     desktop.MouseButtonPrimary,
	})
	if calls != 1 {
		t.Fatalf("drag calls = %d, want 1 after diagonal threshold", calls)
	}

	icon.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{AbsolutePosition: fyne.NewPos(10, 10)},
		Button:     desktop.MouseButtonPrimary,
	})
	icon.MouseMoved(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{AbsolutePosition: fyne.NewPos(16, 12)},
		Button:     desktop.MouseButtonPrimary,
	})
	if calls != 2 {
		t.Fatalf("drag calls = %d, want 2 after axis threshold", calls)
	}
}
