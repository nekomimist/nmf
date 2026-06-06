package ui

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	customtheme "nmf/internal/theme"
)

type busyOverlayTheme struct{}

func (busyOverlayTheme) GetCustomColor(colorType string) color.RGBA {
	if colorType == customtheme.ColorBusyOverlayBackground {
		return color.RGBA{0, 0, 0, 96}
	}
	return color.RGBA{}
}

func TestBusyOverlayLongTextDoesNotExpandMinWidth(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	overlay := NewBusyOverlay(busyOverlayTheme{})
	longText := "Loading smb://server/share/" + strings.Repeat("very-long-path-segment/", 40) + "file.tar.xz..."

	overlay.Show(nil, longText)

	textWidth := fyne.MeasureText(longText, theme.TextSize(), fyne.TextStyle{}).Width
	minWidth := overlay.GetContainer().MinSize().Width
	if minWidth >= textWidth/4 {
		t.Fatalf("busy overlay min width = %.2f, want much less than full text width %.2f", minWidth, textWidth)
	}
}

func TestBusyOverlayRestartsSpinnerWhenShownAgain(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	overlay := NewBusyOverlay(busyOverlayTheme{})

	overlay.Show(nil, "Loading first...")
	if !overlay.spinner.Running() {
		t.Fatal("spinner should be running after show")
	}

	overlay.Hide()
	if overlay.spinner.Running() {
		t.Fatal("spinner should stop after hide")
	}

	overlay.Show(nil, "Loading second...")
	if !overlay.spinner.Running() {
		t.Fatal("spinner should restart after showing again")
	}
}
