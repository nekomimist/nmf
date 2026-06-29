package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"nmf/internal/filecompare"
)

func TestCompactComparePathKeepsShortPath(t *testing.T) {
	path := "smb://host/share/data"
	if got := compactComparePath(path, 72); got != path {
		t.Fatalf("compactComparePath(%q) = %q, want unchanged", path, got)
	}
}

func TestCompactComparePathShortensLongSlashPath(t *testing.T) {
	path := "smb://naja.local/neko/sd-scripts/Data/train/1_nm loli11"
	got := compactComparePath(path, 32)

	if utf8.RuneCountInString(got) > 32 {
		t.Fatalf("compact path length = %d, want <= 32: %q", utf8.RuneCountInString(got), got)
	}
	if !strings.Contains(got, "/.../") {
		t.Fatalf("compact path = %q, want slash ellipsis marker", got)
	}
	if !strings.HasPrefix(got, "smb://") {
		t.Fatalf("compact path = %q, want source prefix", got)
	}
	if !strings.HasSuffix(got, "1_nm loli11") {
		t.Fatalf("compact path = %q, want source suffix", got)
	}
}

func TestCompactComparePathUsesWindowsMarker(t *testing.T) {
	path := `C:\Users\hiro\work\StableDiffusion\txt2img-images`
	got := compactComparePath(path, 30)

	if !strings.Contains(got, `\...\`) {
		t.Fatalf("compact path = %q, want windows ellipsis marker", got)
	}
	if utf8.RuneCountInString(got) > 30 {
		t.Fatalf("compact path length = %d, want <= 30: %q", utf8.RuneCountInString(got), got)
	}
}

func TestCompareSourcePathMaxRunesForWidthKeepsMinimum(t *testing.T) {
	got := compareSourcePathMaxRunesForWidth(compareDialogWidth)

	if got < compareSourcePathMaxRunes {
		t.Fatalf("max runes = %d, want at least %d", got, compareSourcePathMaxRunes)
	}
}

func TestCompareSourcePathMaxRunesForWidthExpandsWithWidth(t *testing.T) {
	narrow := compareSourcePathMaxRunesForWidth(compareDialogWidth)
	wide := compareSourcePathMaxRunesForWidth(compareDialogWidth * 2)

	if wide <= narrow {
		t.Fatalf("wide max runes = %d, want greater than narrow %d", wide, narrow)
	}
}

func TestCompareMethodLabelsShowAltShortcuts(t *testing.T) {
	want := []string{
		"Missing in destination or newer (Alt+U)",
		"Missing in destination (Alt+M)",
		"Newer than destination (Alt+N)",
		"File size matches (Alt+S)",
		"File size and timestamp match (Alt+T)",
		"File size and content match (Alt+C)",
	}

	got := compareMethodLabels()
	if len(got) != len(want) {
		t.Fatalf("compare method labels length = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("compare method label[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCompareDialogMethodRadioRequiresSelection(t *testing.T) {
	dialog := NewCompareDialog("/src", 1, nil, nil, func(string, ...interface{}) {})

	if dialog.methodRadio == nil {
		t.Fatal("compare method radio group was not created")
	}
	if !dialog.methodRadio.Required {
		t.Fatal("compare method radio group should require one selected option")
	}
	if got := dialog.methodRadio.Selected; got != compareMethodLabel(filecompare.MissingOrNewer) {
		t.Fatalf("default compare method = %q, want %q", got, compareMethodLabel(filecompare.MissingOrNewer))
	}
}
