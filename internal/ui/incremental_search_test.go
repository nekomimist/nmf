package ui

import (
	"image/color"
	"strings"
	"testing"

	"nmf/internal/fileinfo"
	customtheme "nmf/internal/theme"
)

type incrementalSearchTheme struct{}

func (incrementalSearchTheme) GetCustomColor(colorType string) color.RGBA {
	switch colorType {
	case customtheme.ColorSearchOverlayBackground:
		return color.RGBA{40, 40, 40, 240}
	case customtheme.ColorSearchOverlayForeground:
		return color.RGBA{255, 255, 255, 255}
	default:
		return color.RGBA{}
	}
}

func TestIncrementalSearchShowUsesShortInitialPrompt(t *testing.T) {
	overlay := NewIncrementalSearchOverlay([]fileinfo.FileInfo{
		{Name: "alpha.txt"},
		{Name: "beta.txt"},
	}, nil, incrementalSearchTheme{}, func(string, ...interface{}) {})

	overlay.Show(nil)

	text := overlay.searchLabel.Text
	if !strings.Contains(text, "Type to narrow down") {
		t.Fatalf("initial prompt %q should ask to narrow down", text)
	}
	if strings.Contains(text, "Incremental Search - Type to search files") {
		t.Fatalf("initial prompt %q should not use the long instructional text", text)
	}
	if strings.Contains(text, "navigate") || strings.Contains(text, "Enter") || strings.Contains(text, "Esc") {
		t.Fatalf("initial prompt %q should not include keyboard help", text)
	}
}

func TestIncrementalSearchShowSelectsFirstFileWhenSearchIsEmpty(t *testing.T) {
	overlay := NewIncrementalSearchOverlay([]fileinfo.FileInfo{
		{Name: "alpha.txt"},
		{Name: "beta.txt"},
	}, nil, incrementalSearchTheme{}, func(string, ...interface{}) {})

	overlay.Show(nil)

	match := overlay.GetCurrentMatch()
	if match == nil {
		t.Fatal("expected empty search to select the first file")
	}
	if match.Name != "alpha.txt" {
		t.Fatalf("current match got %q, want alpha.txt", match.Name)
	}
}

func TestIncrementalSearchTypingUpdatesMatchDisplay(t *testing.T) {
	overlay := NewIncrementalSearchOverlay([]fileinfo.FileInfo{
		{Name: "alpha.txt"},
		{Name: "beta.txt"},
	}, nil, incrementalSearchTheme{}, func(string, ...interface{}) {})

	overlay.Show(nil)
	overlay.AddCharacter('b')

	text := overlay.searchLabel.Text
	if !strings.Contains(text, "Search: b [1/1]") {
		t.Fatalf("search display got %q, want typed term and match count", text)
	}
	match := overlay.GetCurrentMatch()
	if match == nil || match.Name != "beta.txt" {
		t.Fatalf("current match got %+v, want beta.txt", match)
	}
}

func TestIncrementalSearchLongMatchDoesNotExpandOverlayMinSize(t *testing.T) {
	overlay := NewIncrementalSearchOverlay([]fileinfo.FileInfo{
		{Name: strings.Repeat("long-name-", 80) + ".txt"},
	}, nil, incrementalSearchTheme{}, func(string, ...interface{}) {})

	before := overlay.GetContainer().MinSize().Width
	overlay.Show(nil)
	overlay.AddCharacter('l')
	after := overlay.GetContainer().MinSize().Width

	if after > before {
		t.Fatalf("overlay min width grew from %.2f to %.2f for long match name", before, after)
	}
}

func TestIncrementalSearchTextUsesExplicitContrastColor(t *testing.T) {
	overlay := NewIncrementalSearchOverlay([]fileinfo.FileInfo{{Name: "alpha.txt"}}, nil, incrementalSearchTheme{}, func(string, ...interface{}) {})

	got := color.RGBAModel.Convert(overlay.searchLabel.Color).(color.RGBA)
	want := color.RGBA{255, 255, 255, 255}
	if got != want {
		t.Fatalf("search text color = %#v, want explicit contrast color %#v", got, want)
	}
}
