package ui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	fynetheme "fyne.io/fyne/v2/theme"

	"nmf/internal/config"
	"nmf/internal/keymanager"
	customtheme "nmf/internal/theme"
)

type lineEditEntryTestAdapter struct {
	entry *LineEditEntry
}

func (a lineEditEntryTestAdapter) AcceptEdit()                {}
func (a lineEditEntryTestAdapter) CancelDialog()              {}
func (a lineEditEntryTestAdapter) MoveCursorStart()           { a.entry.MoveCursorStart() }
func (a lineEditEntryTestAdapter) MoveCursorEnd()             { a.entry.MoveCursorEnd() }
func (a lineEditEntryTestAdapter) MoveCursorLeft()            { a.entry.MoveCursorLeft() }
func (a lineEditEntryTestAdapter) MoveCursorRight()           { a.entry.MoveCursorRight() }
func (a lineEditEntryTestAdapter) DeleteBeforeCursor()        { a.entry.DeleteBeforeCursor() }
func (a lineEditEntryTestAdapter) DeleteAtCursor()            { a.entry.DeleteAtCursor() }
func (a lineEditEntryTestAdapter) DeleteBeforeCursorToStart() { a.entry.DeleteBeforeCursorToStart() }
func (a lineEditEntryTestAdapter) DeleteAfterCursorToEnd()    { a.entry.DeleteAfterCursorToEnd() }
func (a lineEditEntryTestAdapter) PasteFromClipboard()        { a.entry.PasteFromClipboard() }
func (a lineEditEntryTestAdapter) InsertRune(r rune) bool {
	a.entry.InsertText(string(r))
	return true
}

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

func TestLineEditEntrySelectRange(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("note.txt")

	entry.SelectRange(0, 4)

	if got := entry.SelectedText(); got != "note" {
		t.Fatalf("SelectedText() = %q, want note", got)
	}
	if entry.CursorColumn != 4 {
		t.Fatalf("cursor = %d, want 4", entry.CursorColumn)
	}
}

func TestLineEditEntrySelectRangeUsesRuneOffsets(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("日本語.txt")

	entry.SelectRange(0, 3)

	if got := entry.SelectedText(); got != "日本語" {
		t.Fatalf("SelectedText() = %q, want 日本語", got)
	}
	if entry.CursorColumn != 3 {
		t.Fatalf("cursor = %d, want 3", entry.CursorColumn)
	}
}

func TestLineEditEntryMoveClearsSelection(t *testing.T) {
	tests := []struct {
		name       string
		move       func(*LineEditEntry)
		wantCursor int
	}{
		{name: "start", move: (*LineEditEntry).MoveCursorStart, wantCursor: 0},
		{name: "end", move: (*LineEditEntry).MoveCursorEnd, wantCursor: 8},
		{name: "left", move: (*LineEditEntry).MoveCursorLeft, wantCursor: 0},
		{name: "right", move: (*LineEditEntry).MoveCursorRight, wantCursor: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := NewLineEditEntry(nil)
			entry.SetText("note.txt")
			entry.SelectRange(0, 4)

			tt.move(entry)

			if got := entry.SelectedText(); got != "" {
				t.Fatalf("SelectedText() = %q, want empty", got)
			}
			if entry.CursorColumn != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", entry.CursorColumn, tt.wantCursor)
			}
		})
	}
}

func TestLineEditEntryMoveClearsSelectionWithoutSyntheticShiftKeyUp(t *testing.T) {
	tests := []struct {
		name       string
		key        fyne.KeyName
		wantCursor int
	}{
		{name: "left", key: fyne.KeyLeft, wantCursor: 0},
		{name: "right", key: fyne.KeyRight, wantCursor: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs []string
			km := keymanager.NewKeyManager(func(format string, args ...interface{}) {
				logs = append(logs, fmt.Sprintf(format, args...))
			})
			entry := NewLineEditEntry(nil, km)
			entry.SetText("note.txt")
			entry.SelectRange(0, 4)
			handler := keymanager.NewLineEditDialogKeyHandler(lineEditEntryTestAdapter{entry: entry}, km.Debugf)
			km.PushHandler(handler)
			logs = nil

			entry.KeyDown(&fyne.KeyEvent{Name: tt.key})
			entry.TypedKey(&fyne.KeyEvent{Name: tt.key})
			entry.KeyUp(&fyne.KeyEvent{Name: tt.key})

			if got := entry.SelectedText(); got != "" {
				t.Fatalf("SelectedText() = %q, want empty", got)
			}
			if entry.CursorColumn != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", entry.CursorColumn, tt.wantCursor)
			}
			joined := strings.Join(logs, "\n")
			if !strings.Contains(joined, "KeyManager: KeyActivated LineEditDialog handled=true") {
				t.Fatalf("logs do not show handled LineEdit activation:\n%s", joined)
			}
			if strings.Contains(joined, "KeyManager: Shift up") || strings.Contains(joined, "key=LeftShift") {
				t.Fatalf("move emitted synthetic shift release logs:\n%s", joined)
			}
		})
	}
}

func TestLineEditDialogDefaultsToEndCursorWithoutInitialSelection(t *testing.T) {
	dialog := NewLineEditDialog(LineEditDialogOptions{InitialText: "abc"}, nil)

	if dialog.entry.CursorColumn != 3 {
		t.Fatalf("cursor = %d, want end", dialog.entry.CursorColumn)
	}
	if got := dialog.entry.SelectedText(); got != "" {
		t.Fatalf("SelectedText() = %q, want empty", got)
	}
}

func TestLineEditDialogAppliesInitialSelection(t *testing.T) {
	dialog := NewLineEditDialog(LineEditDialogOptions{
		InitialText:      "note.txt",
		InitialSelection: &LineEditSelection{Start: 0, End: 4},
	}, nil)

	if got := dialog.entry.SelectedText(); got != "note" {
		t.Fatalf("SelectedText() = %q, want note", got)
	}
	if dialog.entry.CursorColumn != 4 {
		t.Fatalf("cursor = %d, want 4", dialog.entry.CursorColumn)
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

func TestLineEditEntryConfiguredNoopDoesNotFallBackToReadlineShortcut(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	entry := NewLineEditEntry(nil, km)
	entry.SetText("abcd")
	entry.MoveCursorEnd()
	handler := keymanager.NewLineEditDialogKeyHandler(lineEditEntryTestAdapter{entry: entry}, km.Debugf, []config.KeyBindingEntry{
		{Target: keymanager.KeyBindingTargetLineEdit, Key: "C-A", Command: keymanager.CommandNoop},
	})
	km.PushHandler(handler)

	entry.KeyDown(&fyne.KeyEvent{Name: fyne.KeyA})
	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyA, Modifier: fyne.KeyModifierControl})

	if entry.CursorColumn != 4 {
		t.Fatalf("cursor after configured noop ctrl-a = %d, want 4", entry.CursorColumn)
	}
}

func TestLineEditEntryKillBeforeCursorCopiesToClipboard(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(3)

	entry.DeleteBeforeCursorToStart()

	if got := app.Clipboard().Content(); got != "abc" {
		t.Fatalf("clipboard = %q, want abc", got)
	}
	if entry.Text != "d" {
		t.Fatalf("text = %q, want d", entry.Text)
	}
	if entry.CursorColumn != 0 {
		t.Fatalf("cursor = %d, want 0", entry.CursorColumn)
	}
}

func TestLineEditEntryKillAfterCursorCopiesToClipboard(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(1)

	entry.DeleteAfterCursorToEnd()

	if got := app.Clipboard().Content(); got != "bcd" {
		t.Fatalf("clipboard = %q, want bcd", got)
	}
	if entry.Text != "a" {
		t.Fatalf("text = %q, want a", entry.Text)
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryCtrlYPastesClipboard(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Clipboard().SetContent("bc")
	entry := NewLineEditEntry(nil)
	entry.SetText("ad")
	entry.setCursor(1)

	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyY, Modifier: fyne.KeyModifierControl})

	if entry.Text != "abcd" {
		t.Fatalf("text = %q, want abcd", entry.Text)
	}
	if entry.CursorColumn != 3 {
		t.Fatalf("cursor = %d, want 3", entry.CursorColumn)
	}
}

func TestLineEditEntryPasteFromClipboardFlattensNewlines(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Clipboard().SetContent("b\nc")
	entry := NewLineEditEntry(nil)
	entry.SetText("ad")
	entry.setCursor(1)

	entry.PasteFromClipboard()

	if entry.Text != "ab cd" {
		t.Fatalf("text = %q, want ab cd", entry.Text)
	}
	if entry.CursorColumn != 4 {
		t.Fatalf("cursor = %d, want 4", entry.CursorColumn)
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

	// Capture the stock widget.Entry layout (which applies its own baseline
	// border/padding) before our inset is layered on, so the inset delta can
	// be checked without assuming Fyne's internal padding constants.
	renderer.base.Layout(size)
	content := renderer.contentObject()
	if content == nil {
		t.Fatal("line edit content object was not found")
	}
	beforeHeight := content.Size().Height

	renderer.applyContentInset()
	renderer.updateCaret()

	if content.Position().X < lineEditEntryHorizontalInset {
		t.Fatalf("content x inset = %f, want at least %f", content.Position().X, lineEditEntryHorizontalInset)
	}
	if wantHeight := beforeHeight - lineEditEntryVerticalInset*2; content.Size().Height != wantHeight {
		t.Fatalf("content height = %f, want %f (before-inset height minus vertical inset on both sides)", content.Size().Height, wantHeight)
	}

	caretRight := renderer.caret.Position().X + renderer.caret.Size().Width
	maxRight := size.Width - lineEditEntryHorizontalInset
	if caretRight > maxRight {
		t.Fatalf("caret right edge = %f, want at most %f", caretRight, maxRight)
	}
}

// TestLineEditEntryRendererAppliesSymmetricVerticalInset guards against the
// vertical text offset regression: applyContentInset must shift/shrink the
// content by lineEditEntryVerticalInset on both axes, matching MinSize()'s
// symmetric inset*2 addition. The embedded stock widget.Entry renderer adds
// its own baseline offset before this runs, so the assertion checks the
// delta applyContentInset introduces rather than an absolute position.
func TestLineEditEntryRendererAppliesSymmetricVerticalInset(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	entry := NewIMEEntry(nil)
	w := test.NewWindow(entry)
	defer w.Close()

	renderer := test.WidgetRenderer(entry).(*lineEditEntryRenderer)
	size := renderer.MinSize()
	entry.Resize(size)

	renderer.base.Layout(size)
	content := renderer.contentObject()
	if content == nil {
		t.Fatal("line edit content object was not found")
	}
	beforePos := content.Position()
	beforeSize := content.Size()

	renderer.applyContentInset()

	wantPos := beforePos.Add(fyne.NewPos(lineEditEntryHorizontalInset, lineEditEntryVerticalInset))
	if content.Position() != wantPos {
		t.Fatalf("content position = %v, want %v", content.Position(), wantPos)
	}

	wantSize := beforeSize.Subtract(fyne.NewSize(lineEditEntryHorizontalInset*2, lineEditEntryVerticalInset*2))
	if content.Size() != wantSize {
		t.Fatalf("content size = %v, want %v", content.Size(), wantSize)
	}
}
