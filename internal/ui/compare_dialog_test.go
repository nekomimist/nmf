package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	fynetest "fyne.io/fyne/v2/test"

	"nmf/internal/filecompare"
	"nmf/internal/keymanager"
)

func TestCompactComparePathKeepsShortPath(t *testing.T) {
	path := "smb://host/share/data"
	if got := compactComparePath(path, 72); got != path {
		t.Fatalf("compactComparePath(%q) = %q, want unchanged", path, got)
	}
}

func TestCompareDialogOrderChangeKeepsMethodAndSinkFocus(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()
	window := app.NewWindow("compare")
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewCompareDialog("/src", 1, 0, nil, km, func(string, ...interface{}) {})
	d.SelectSizeContentEqual()
	d.ShowDialog(window, func(CompareResult) {})
	defer d.dialog.Hide()
	defer km.RemoveHandler(d.kmToken)

	lastAccess := fynetest.WidgetRenderer(d.orderRadio).Objects()[1]
	window.Canvas().Focus(lastAccess.(fyne.Focusable))
	fynetest.Tap(lastAccess.(fyne.Tappable))
	if d.orderRadio.Selected != navigationHistoryOrderLastUsed {
		t.Fatal("tapping Last Access should change the order mode even with no candidates")
	}
	if d.selectedMethod() != filecompare.SizeContentEqual {
		t.Fatal("changing destination order should preserve the comparison method")
	}
	if focused := window.Canvas().Focused(); focused != d.sink {
		t.Fatalf("focus after order change = %T, want compare dialog sink", focused)
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

func TestCompareDialogDefaultMethod(t *testing.T) {
	tests := []struct {
		name    string
		files   int
		dirs    int
		method  filecompare.Method
		enabled bool
	}{
		{name: "files and subdirectories", files: 1, dirs: 1, method: filecompare.MissingOrNewer, enabled: true},
		{name: "only subdirectories", dirs: 1, method: filecompare.MissingSubdirectory, enabled: true},
		{name: "empty", method: filecompare.MissingOrNewer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewCompareDialog("/src", tt.files, tt.dirs, nil, nil, func(string, ...interface{}) {})
			if !d.methodRadio.Required {
				t.Fatal("compare method radio group should require one selected option")
			}
			if got := d.selectedMethod(); got != tt.method {
				t.Fatalf("default compare method = %q, want %q", got, tt.method)
			}
			if enabled := !d.compareButton.Disabled(); enabled != tt.enabled {
				t.Fatalf("Compare button enabled = %t, want %t", enabled, tt.enabled)
			}
		})
	}
}

func TestCompareDialogBlocksConfirmationWithoutSourceEntries(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()
	window := app.NewWindow("compare")
	km := keymanager.NewKeyManager(nil)
	destination := t.TempDir()
	d := NewCompareDialog("/src", 0, 1, []DestinationCandidate{{Path: destination}}, km, func(string, ...interface{}) {})
	var accepted CompareResult
	d.ShowDialog(window, func(result CompareResult) { accepted = result })
	defer d.CancelDialog()
	handler := km.GetCurrentHandler()

	handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyM}, keymanager.ModifierState{AltPressed: true})
	if !d.compareButton.Disabled() {
		t.Fatal("Compare button should be disabled for a file method with no files")
	}
	for _, modifiers := range []keymanager.ModifierState{{}, {CtrlPressed: true}} {
		if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, modifiers) || d.closed || accepted.Destination != "" {
			t.Fatalf("confirmation with ctrl=%t should be handled and leave the dialog open", modifiers.CtrlPressed)
		}
	}

	handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyD}, keymanager.ModifierState{AltPressed: true})
	if d.compareButton.Disabled() {
		t.Fatal("Compare button should be enabled for a subdirectory method with subdirectories")
	}
	handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, keymanager.ModifierState{})
	if accepted != (CompareResult{Destination: destination, Method: filecompare.MissingSubdirectory}) {
		t.Fatalf("accepted = %+v, want subdirectory comparison to %s", accepted, destination)
	}
}
