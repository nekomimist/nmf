package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	fynetheme "fyne.io/fyne/v2/theme"

	"nmf/internal/config"
	customtheme "nmf/internal/theme"
)

func TestLineEditEntryReadlineCursorMovement(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abc")
	entry.MoveCursorEnd()

	entry.MoveCursorLeft()
	if entry.CursorColumn != 2 {
		t.Fatalf("cursor after left = %d, want 2", entry.CursorColumn)
	}
	entry.MoveCursorStart()
	if entry.CursorColumn != 0 {
		t.Fatalf("cursor after start = %d, want 0", entry.CursorColumn)
	}
	entry.MoveCursorRight()
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor after right = %d, want 1", entry.CursorColumn)
	}
	entry.MoveCursorEnd()
	if entry.CursorColumn != 3 {
		t.Fatalf("cursor after end = %d, want 3", entry.CursorColumn)
	}
}

func TestLineEditEntryDeleteBeforeCursor(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(2)

	entry.DeleteBeforeCursor()

	if entry.Text != "acd" {
		t.Fatalf("text = %q, want %q", entry.Text, "acd")
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryDeleteAtCursor(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(1)

	entry.DeleteAtCursor()

	if entry.Text != "acd" {
		t.Fatalf("text = %q, want %q", entry.Text, "acd")
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryDeleteBeforeCursorToStart(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(3)

	entry.DeleteBeforeCursorToStart()

	if entry.Text != "d" {
		t.Fatalf("text = %q, want %q", entry.Text, "d")
	}
	if entry.CursorColumn != 0 {
		t.Fatalf("cursor = %d, want 0", entry.CursorColumn)
	}
}

func TestLineEditEntryDeleteAfterCursorToEnd(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(1)

	entry.DeleteAfterCursorToEnd()

	if entry.Text != "a" {
		t.Fatalf("text = %q, want %q", entry.Text, "a")
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryHandlesRunes(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("あいう")
	entry.setCursor(2)

	entry.DeleteBeforeCursor()

	if entry.Text != "あう" {
		t.Fatalf("text = %q, want %q", entry.Text, "あう")
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryInsertTextAtCursor(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("acd")
	entry.setCursor(1)

	entry.InsertText("b")

	if entry.Text != "abcd" {
		t.Fatalf("text = %q, want %q", entry.Text, "abcd")
	}
	if entry.CursorColumn != 2 {
		t.Fatalf("cursor = %d, want 2", entry.CursorColumn)
	}
}

func TestLineEditEntryReadlineShortcutKeys(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.MoveCursorEnd()

	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyA, Modifier: fyne.KeyModifierControl})

	if entry.CursorColumn != 0 {
		t.Fatalf("cursor after ctrl-a = %d, want 0", entry.CursorColumn)
	}

	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyE, Modifier: fyne.KeyModifierControl})
	if entry.CursorColumn != 4 {
		t.Fatalf("cursor after ctrl-e = %d, want 4", entry.CursorColumn)
	}

	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyH, Modifier: fyne.KeyModifierControl})
	if entry.Text != "abc" {
		t.Fatalf("text after ctrl-h = %q, want %q", entry.Text, "abc")
	}
}

func TestLineEditEntrySelectAllShortcutMovesToStart(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.MoveCursorEnd()

	entry.TypedShortcut(&fyne.ShortcutSelectAll{})

	if entry.CursorColumn != 0 {
		t.Fatalf("cursor after shortcut select-all = %d, want 0", entry.CursorColumn)
	}
}

func TestLineEditEntryReadlineShortcutRepeats(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.MoveCursorEnd()

	shortcut := &desktop.CustomShortcut{KeyName: fyne.KeyH, Modifier: fyne.KeyModifierControl}
	entry.TypedShortcut(shortcut)
	entry.TypedShortcut(shortcut)
	entry.TypedShortcut(shortcut)

	if entry.Text != "a" {
		t.Fatalf("text after repeated ctrl-h = %q, want %q", entry.Text, "a")
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor after repeated ctrl-h = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryReadlineCursorShortcutRepeats(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.MoveCursorEnd()

	left := &desktop.CustomShortcut{KeyName: fyne.KeyB, Modifier: fyne.KeyModifierControl}
	entry.TypedShortcut(left)
	entry.TypedShortcut(left)

	if entry.CursorColumn != 2 {
		t.Fatalf("cursor after repeated ctrl-b = %d, want 2", entry.CursorColumn)
	}

	right := &desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierControl}
	entry.TypedShortcut(right)
	entry.TypedShortcut(right)

	if entry.CursorColumn != 4 {
		t.Fatalf("cursor after repeated ctrl-f = %d, want 4", entry.CursorColumn)
	}
}

func TestLineEditEntryReadlineShortcutDoesNotDoubleApplyAfterKeyDown(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.MoveCursorEnd()

	entry.KeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	entry.KeyDown(&fyne.KeyEvent{Name: fyne.KeyH})
	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyH, Modifier: fyne.KeyModifierControl})

	if entry.Text != "abc" {
		t.Fatalf("text after keydown plus shortcut ctrl-h = %q, want %q", entry.Text, "abc")
	}
}

func TestLineEditEntryCursorColorDoesNotRecolorFocusedBorder(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	cfg := &config.Config{
		Theme: config.ThemeConfig{
			Colors: map[string]config.ThemeColorConfig{
				customtheme.ColorLineEditCursor: {
					Value: &config.ThemeColorValue{RGBA: [4]uint8{255, 255, 0, 255}, IsRGBA: true},
				},
			},
		},
	}
	appTheme := customtheme.NewCustomTheme(cfg, func(string, ...interface{}) {})
	app.Settings().SetTheme(appTheme)

	entry := NewLineEditEntry(nil)
	w := test.NewWindow(lineEditThemeOverride(entry))
	defer w.Close()
	w.Canvas().Focus(entry)

	renderer := test.WidgetRenderer(entry).(*lineEditEntryRenderer)
	renderer.Refresh()
	border := renderer.borderRectangle()
	if border == nil {
		t.Fatal("line edit border rectangle was not found")
	}

	got := color.RGBAModel.Convert(border.StrokeColor)
	want := color.RGBAModel.Convert(appTheme.Color(fynetheme.ColorNamePrimary, app.Settings().ThemeVariant()))
	cursor := color.RGBAModel.Convert(appTheme.GetCustomColor(customtheme.ColorLineEditCursor))
	if got != want {
		t.Fatalf("focused border color = %#v, want app primary %#v", got, want)
	}
	if got == cursor {
		t.Fatalf("focused border color should not use line edit cursor color %#v", cursor)
	}
	if caret := color.RGBAModel.Convert(renderer.caret.FillColor); caret != cursor {
		t.Fatalf("line edit caret color = %#v, want cursor color %#v", caret, cursor)
	}
}

func TestLineEditEntryRendererAddsContentInsetAndClampsCaret(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	entry := NewLineEditEntry(nil)
	entry.SetText("drag_source_state_windows.go!!!0000IIIIaaaaaaaaaaaaaaaa")
	entry.MoveCursorEnd()

	w := test.NewWindow(lineEditThemeOverride(entry))
	defer w.Close()
	w.Canvas().Focus(entry)

	renderer := test.WidgetRenderer(entry).(*lineEditEntryRenderer)
	size := fyne.NewSize(220, renderer.MinSize().Height)
	entry.Resize(size)
	renderer.Layout(size)

	content := renderer.contentObject()
	if content == nil {
		t.Fatal("line edit content object was not found")
	}
	if content.Position().X < lineEditEntryHorizontalInset {
		t.Fatalf("content x inset = %f, want at least %f", content.Position().X, lineEditEntryHorizontalInset)
	}
	if content.Position().Y < lineEditEntryVerticalInset {
		t.Fatalf("content y inset = %f, want at least %f", content.Position().Y, lineEditEntryVerticalInset)
	}

	caretRight := renderer.caret.Position().X + renderer.caret.Size().Width
	maxRight := size.Width - lineEditEntryHorizontalInset
	if caretRight > maxRight {
		t.Fatalf("caret right edge = %f, want at most %f", caretRight, maxRight)
	}
}
