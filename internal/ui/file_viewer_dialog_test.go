package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

func TestViewerTextDisablesBinaryPreview(t *testing.T) {
	preview := &fileinfo.PreviewFile{
		Data:   []byte{0x00, 0x01, 'a'},
		Text:   "\x00\x01a",
		Binary: true,
	}

	got := viewerText(preview)
	if strings.Contains(got, "\x00") || !strings.Contains(got, "Binary file") {
		t.Fatalf("viewerText() = %q, want binary placeholder without raw NUL", got)
	}
}

func TestViewerTextTruncatesDisplayText(t *testing.T) {
	preview := &fileinfo.PreviewFile{
		Text: strings.Repeat("a", fileViewerTextLimit+1),
	}

	got := viewerText(preview)
	if !strings.Contains(got, "viewer text truncated") {
		t.Fatalf("viewerText() missing truncation notice")
	}
	if !strings.Contains(got, fileinfo.FormatFileSize(int64(fileViewerTextLimit))) {
		t.Fatalf("viewerText() missing text limit")
	}
}

func TestViewerTextEscapesNonPrintableText(t *testing.T) {
	preview := &fileinfo.PreviewFile{
		Text: "hello\x1b[31m\t日本語\nzero-width:\u200b\x7f",
	}

	got := viewerText(preview)
	if strings.Contains(got, "\x1b") || strings.Contains(got, "\u200b") || strings.Contains(got, "\x7f") {
		t.Fatalf("viewerText() = %q, want raw non-printable characters escaped", got)
	}
	for _, want := range []string{`hello\u001B[31m`, "\t日本語\n", `zero-width:\u200B\u007F`} {
		if !strings.Contains(got, want) {
			t.Fatalf("viewerText() = %q, want fragment %q", got, want)
		}
	}
}

func TestViewerTextKeepsIdeographicSpace(t *testing.T) {
	preview := &fileinfo.PreviewFile{
		Text: "a\u3000b",
	}

	got := viewerText(preview)
	if got != "a\u3000b" {
		t.Fatalf("viewerText() = %q, want ideographic space preserved", got)
	}
}

func TestViewerHexTruncatesDisplayData(t *testing.T) {
	preview := &fileinfo.PreviewFile{
		Data: bytes.Repeat([]byte{0xff}, fileViewerHexLimit+1),
	}

	got := viewerHex(preview)
	if !strings.Contains(got, "viewer hex truncated") {
		t.Fatalf("viewerHex() missing truncation notice")
	}
	if strings.Contains(got, fmt.Sprintf("%08x", hexDumpDataLimit(fileViewerHexLimit))) {
		t.Fatalf("viewerHex() includes offset beyond display limit")
	}
}

func TestTruncateUTF8BytesKeepsValidUTF8(t *testing.T) {
	got, truncated := truncateUTF8Bytes("abc日本語", 5)
	if !truncated {
		t.Fatal("truncateUTF8Bytes() truncated = false, want true")
	}
	if !utf8.ValidString(got) {
		t.Fatalf("truncateUTF8Bytes() returned invalid UTF-8: %q", got)
	}
	if got != "abc" {
		t.Fatalf("truncateUTF8Bytes() = %q, want %q", got, "abc")
	}
}

func TestFileViewerTextGridTreatsEmptyTextAsOneLine(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("", nil, nil, nil)

	if grid.TotalLines() != 1 {
		t.Fatalf("TotalLines() = %d, want 1", grid.TotalLines())
	}
	if grid.CurrentLine() != 1 {
		t.Fatalf("CurrentLine() = %d, want 1", grid.CurrentLine())
	}
}

func TestFileViewerTextGridClampsMovement(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("1\n2\n3\n4\n5", nil, nil, nil)
	grid.visibleRows = 2

	grid.MoveRows(10)
	if grid.CurrentLine() != 5 {
		t.Fatalf("CurrentLine() after MoveRows(10) = %d, want 5", grid.CurrentLine())
	}
	grid.MoveRows(-10)
	if grid.CurrentLine() != 1 {
		t.Fatalf("CurrentLine() after MoveRows(-10) = %d, want 1", grid.CurrentLine())
	}
}

func TestFileViewerTextGridHorizontalScrollClampsAndSlices(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("abcdef", nil, nil, nil)
	grid.visibleRows = 1
	grid.visibleCols = 3

	grid.MoveColumns(2)
	if grid.CurrentColumn() != 3 {
		t.Fatalf("CurrentColumn() = %d, want 3", grid.CurrentColumn())
	}
	if got := grid.grid.Text(); got != "cde" {
		t.Fatalf("grid text after horizontal scroll = %q, want %q", got, "cde")
	}

	grid.MoveColumns(99)
	if grid.CurrentColumn() != 4 {
		t.Fatalf("CurrentColumn() after clamp = %d, want 4", grid.CurrentColumn())
	}
	if got := grid.grid.Text(); got != "def" {
		t.Fatalf("grid text after clamp = %q, want %q", got, "def")
	}
}

func TestFileViewerTextGridToggleWrapResetsColumn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("abcdef", nil, nil, nil)
	grid.visibleRows = 2
	grid.visibleCols = 3
	grid.MoveColumns(2)

	if !grid.ToggleWrap() {
		t.Fatal("ToggleWrap() = false, want true")
	}
	if grid.CurrentColumn() != 1 {
		t.Fatalf("CurrentColumn() = %d, want 1 after wrap", grid.CurrentColumn())
	}
	if got := grid.grid.Text(); got != "abc\ndef" {
		t.Fatalf("grid text in wrap mode = %q, want wrapped rows", got)
	}
}

func TestFileViewerTextGridMinSizeDoesNotFollowVisibleLineWidth(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("short\n"+strings.Repeat("x", 200), nil, nil, nil)
	renderer := grid.CreateRenderer()
	before := renderer.MinSize()

	grid.MoveRows(1)
	after := renderer.MinSize()

	if before != after {
		t.Fatalf("MinSize() changed after moving visible text: before=%v after=%v", before, after)
	}
	if after.Width != 0 {
		t.Fatalf("MinSize().Width = %v, want stable zero width", after.Width)
	}
}

func TestFileViewerDialogSizeUsesCanvasRatioWithoutCap(t *testing.T) {
	w := test.NewWindow(widget.NewLabel("parent"))
	w.Resize(fyne.NewSize(2000, 1000))

	got := fileViewerDialogSize(w, 0, 0)

	if got.Width != 1920 || got.Height != 880 {
		t.Fatalf("fileViewerDialogSize() = %v, want 1920x880", got)
	}
}

func TestFileViewerDialogSizeAppliesConfiguredCap(t *testing.T) {
	w := test.NewWindow(widget.NewLabel("parent"))
	w.Resize(fyne.NewSize(2000, 1000))

	got := fileViewerDialogSize(w, 1200, 700)

	if got.Width != 1200 || got.Height != 700 {
		t.Fatalf("fileViewerDialogSize() = %v, want 1200x700", got)
	}
}

func TestFileViewerDialogSizeUsesFallbackWithoutParent(t *testing.T) {
	got := fileViewerDialogSize(nil, 800, 600)

	if got.Width != 800 || got.Height != 600 {
		t.Fatalf("fileViewerDialogSize(nil) = %v, want capped fallback 800x600", got)
	}
}

func TestFileViewerDialogMarkdownStartsOnTextTab(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	d := NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:     "README.md",
		Text:     "# title",
		Markdown: true,
	})
	d.ShowDialog(w)
	defer d.CancelDialog()

	if d.activeName != "Text" {
		t.Fatalf("activeName = %q, want Text", d.activeName)
	}
}

func TestFileViewerDialogBinaryStartsOnHexTextGrid(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	d := NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:   "data.bin",
		Data:   []byte{0x00, 0x01, 0xff},
		Text:   "\x00\x01\xff",
		Binary: true,
	})
	d.ShowDialog(w)
	defer d.CancelDialog()

	if d.activeName != "Hex" {
		t.Fatalf("activeName = %q, want Hex", d.activeName)
	}
	if d.hexGrid == nil {
		t.Fatal("hexGrid is nil, want TextGrid hex viewer")
	}
	if got := strings.Join(d.hexGrid.lines, "\n"); !strings.Contains(got, "00000000") {
		t.Fatalf("hex grid text = %q, want hex dump offset", got)
	}
}

func TestFileViewerDialogLazyLoadsHexTextGrid(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	d := NewFileViewerDialog(&fileinfo.PreviewFile{
		Path: "note.txt",
		Data: []byte("hello"),
		Text: "hello",
	})
	d.ShowDialog(w)
	defer d.CancelDialog()

	if d.hexGrid != nil {
		t.Fatal("hexGrid loaded before Hex tab selection")
	}
	d.tabs.Select(d.hexTab)
	if d.hexGrid == nil {
		t.Fatal("hexGrid is nil after Hex tab selection")
	}
}

func TestFileViewerTextGridPageMovementUsesVisibleRows(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("1\n2\n3\n4\n5\n6", nil, nil, nil)
	grid.visibleRows = 3

	grid.PageDown()
	if grid.CurrentLine() != 3 {
		t.Fatalf("CurrentLine() after PageDown() = %d, want 3", grid.CurrentLine())
	}
	grid.PageUp()
	if grid.CurrentLine() != 1 {
		t.Fatalf("CurrentLine() after PageUp() = %d, want 1", grid.CurrentLine())
	}
}

func TestFileViewerTextGridJumpToLineClampsRange(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("1\n2\n3", nil, nil, nil)

	if got := grid.JumpToLine(99); got != 3 {
		t.Fatalf("JumpToLine(99) = %d, want 3", got)
	}
	if grid.CurrentLine() != 3 {
		t.Fatalf("CurrentLine() = %d, want 3", grid.CurrentLine())
	}
	if got := grid.JumpToLine(-1); got != 1 {
		t.Fatalf("JumpToLine(-1) = %d, want 1", got)
	}
}

func TestFileViewerTextGridSelectedTextSingleLine(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("abcdef", nil, nil, nil)
	grid.selection = viewerTextSelection{
		start: viewerTextPosition{line: 0, col: 1},
		end:   viewerTextPosition{line: 0, col: 4},
		set:   true,
	}

	if got := grid.SelectedText(); got != "bcd" {
		t.Fatalf("SelectedText() = %q, want %q", got, "bcd")
	}
}

func TestFileViewerTextGridSelectedTextMultiLine(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("abc\ndef\nghi", nil, nil, nil)
	grid.selection = viewerTextSelection{
		start: viewerTextPosition{line: 0, col: 1},
		end:   viewerTextPosition{line: 2, col: 2},
		set:   true,
	}

	if got := grid.SelectedText(); got != "bc\ndef\ngh" {
		t.Fatalf("SelectedText() = %q, want multi-line logical text", got)
	}
}

func TestFileViewerTextGridSelectedTextSkipsDisplayPadding(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	line := "a\tあb"
	grid := newFileViewerTextGrid(line, nil, nil, nil)
	grid.selection = viewerTextSelection{
		start: viewerTextPosition{line: 0, col: 0},
		end:   viewerTextPosition{line: 0, col: len([]rune(line))},
		set:   true,
	}

	if got := grid.SelectedText(); got != line {
		t.Fatalf("SelectedText() = %q, want original logical text %q", got, line)
	}
}

func TestFileViewerTextGridPositionForHorizontalScroll(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("abcdef", nil, nil, nil)
	grid.visibleRows = 1
	grid.visibleCols = 3
	grid.cellSize = fyne.NewSize(10, 20)
	grid.MoveColumns(2)

	got := grid.textPositionForCanvasPosition(fyne.NewPos(0, 0))
	want := viewerTextPosition{line: 0, col: 2}
	if got != want {
		t.Fatalf("textPositionForCanvasPosition() = %#v, want %#v", got, want)
	}
}

func TestFileViewerTextGridPositionForWrap(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("abcdef", nil, nil, nil)
	grid.visibleRows = 2
	grid.visibleCols = 3
	grid.cellSize = fyne.NewSize(10, 20)
	grid.ToggleWrap()

	got := grid.textPositionForCanvasPosition(fyne.NewPos(0, 21))
	want := viewerTextPosition{line: 0, col: 3}
	if got != want {
		t.Fatalf("textPositionForCanvasPosition() = %#v, want %#v", got, want)
	}
}

func TestFileViewerDialogCopySelectionUsesClipboard(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	d := NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:     "note.txt",
		Data:     []byte("abcdef"),
		Text:     "abcdef",
		Encoding: "UTF-8",
	})
	d.ShowDialog(w)
	defer d.CancelDialog()

	d.textGrid.selection = viewerTextSelection{
		start: viewerTextPosition{line: 0, col: 1},
		end:   viewerTextPosition{line: 0, col: 4},
		set:   true,
	}
	d.copySelection()

	if got := app.Clipboard().Content(); got != "bcd" {
		t.Fatalf("clipboard = %q, want %q", got, "bcd")
	}
	if !strings.Contains(d.status.Text, "copied=3") {
		t.Fatalf("status = %q, want copied count", d.status.Text)
	}
}

func TestFileViewerDialogTextGridCtrlCCopiesThroughKeyManager(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:     "note.txt",
		Data:     []byte("abcdef"),
		Text:     "abcdef",
		Encoding: "UTF-8",
	}, km)
	d.ShowDialog(w)
	defer d.CancelDialog()

	d.textGrid.selection = viewerTextSelection{
		start: viewerTextPosition{line: 0, col: 1},
		end:   viewerTextPosition{line: 0, col: 4},
		set:   true,
	}
	d.textGrid.KeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	d.textGrid.KeyDown(&fyne.KeyEvent{Name: fyne.KeyC})
	d.textGrid.TypedShortcut(&fyne.ShortcutCopy{})

	if got := app.Clipboard().Content(); got != "bcd" {
		t.Fatalf("clipboard = %q, want %q", got, "bcd")
	}
}

func TestFileViewerTextGridFindForwardAndBackward(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("alpha\nbeta\nalphabet", nil, nil, nil)

	result := grid.Find("alpha", 1)
	if !result.Matched || result.Line != 1 || result.Column != 1 || result.Wrapped {
		t.Fatalf("Find forward = %+v, want first alpha", result)
	}
	result = grid.Find("alpha", 1)
	if !result.Matched || result.Line != 3 || result.Column != 1 || result.Wrapped {
		t.Fatalf("Find next = %+v, want second alpha", result)
	}
	result = grid.Find("alpha", -1)
	if !result.Matched || result.Line != 1 || result.Column != 1 || result.Wrapped {
		t.Fatalf("Find previous = %+v, want first alpha", result)
	}
}

func TestFileViewerTextGridFindWraps(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("one\ntwo\nthree", nil, nil, nil)
	grid.JumpToLine(3)

	result := grid.Find("one", 1)
	if !result.Matched || !result.Wrapped || result.Line != 1 {
		t.Fatalf("Find wrapped = %+v, want wrapped match on line 1", result)
	}
}

func TestFileViewerTextGridFindNotFoundDoesNotMove(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("one\ntwo\nthree", nil, nil, nil)
	grid.JumpToLine(2)

	result := grid.Find("missing", 1)
	if result.Matched {
		t.Fatalf("Find missing = %+v, want no match", result)
	}
	if grid.CurrentLine() != 2 {
		t.Fatalf("CurrentLine() = %d, want unchanged line 2", grid.CurrentLine())
	}
}

func TestFileViewerTextGridFindScrollsHorizontally(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	grid := newFileViewerTextGrid("0123456789abcdef", nil, nil, nil)
	grid.visibleRows = 1
	grid.visibleCols = 4

	result := grid.Find("abc", 1)
	if !result.Matched || result.Column != 11 {
		t.Fatalf("Find = %+v, want match at col 11", result)
	}
	if grid.CurrentColumn() != 11 {
		t.Fatalf("CurrentColumn() = %d, want match column visible", grid.CurrentColumn())
	}
}

func TestFileViewerDialogFindUsesSearchTextLiterally(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	d := NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:     "note.txt",
		Data:     []byte("alpha\n beta"),
		Text:     "alpha\n beta",
		Encoding: "UTF-8",
	})
	d.ShowDialog(w)
	defer d.CancelDialog()

	d.search.SetText(" beta")
	d.findNext()

	if d.textGrid.CurrentLine() != 2 {
		t.Fatalf("CurrentLine() = %d, want literal leading-space match on line 2", d.textGrid.CurrentLine())
	}
	if !strings.Contains(d.status.Text, "match line=2 col=1") {
		t.Fatalf("status = %q, want match location", d.status.Text)
	}
}

func TestFileViewerDialogFindSwitchesMarkdownToText(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	d := NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:     "README.md",
		Data:     []byte("# title\nbody"),
		Text:     "# title\nbody",
		Markdown: true,
		Encoding: "UTF-8",
	})
	d.ShowDialog(w)
	defer d.CancelDialog()

	d.tabs.SelectIndex(1)
	d.activeName = "Markdown"
	d.search.SetText("body")
	d.findNext()

	if d.activeName != "Text" {
		t.Fatalf("activeName = %q, want Text", d.activeName)
	}
	if d.textGrid.CurrentLine() != 2 {
		t.Fatalf("CurrentLine() = %d, want body line", d.textGrid.CurrentLine())
	}
}

func TestFileViewerDialogSlashFocusesSearchAfterKeyRelease(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:     "note.txt",
		Data:     []byte("alpha"),
		Text:     "alpha",
		Encoding: "UTF-8",
	}, km)
	d.ShowDialog(w)
	defer d.CancelDialog()

	d.textGrid.KeyDown(&fyne.KeyEvent{Name: fyne.KeySlash})
	d.textGrid.TypedKey(&fyne.KeyEvent{Name: fyne.KeySlash})
	if w.Canvas().Focused() == d.search {
		t.Fatal("search focused before slash key release")
	}

	d.textGrid.KeyUp(&fyne.KeyEvent{Name: fyne.KeySlash})
	if w.Canvas().Focused() != d.search {
		t.Fatal("search not focused after slash key release")
	}
	if d.search.Text != "" {
		t.Fatalf("search text = %q, want empty after slash command", d.search.Text)
	}
}

func TestFileViewerDialogColonFocusesLineAfterKeyRelease(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:     "note.txt",
		Data:     []byte("alpha"),
		Text:     "alpha",
		Encoding: "UTF-8",
	}, km)
	d.ShowDialog(w)
	defer d.CancelDialog()

	d.textGrid.KeyDown(&fyne.KeyEvent{Name: fyne.KeySemicolon})
	d.textGrid.TypedRune(':')
	if w.Canvas().Focused() == d.jump {
		t.Fatal("line entry focused before colon key release")
	}

	d.textGrid.KeyUp(&fyne.KeyEvent{Name: fyne.KeySemicolon})
	if w.Canvas().Focused() != d.jump {
		t.Fatal("line entry not focused after colon key release")
	}
	if d.jump.Text != "" {
		t.Fatalf("line text = %q, want empty after colon command", d.jump.Text)
	}
}

func TestSplitViewerLinesKeepsTrailingEmptyLine(t *testing.T) {
	lines := splitViewerLines("a\n")
	if len(lines) != 2 || lines[0] != "a" || lines[1] != "" {
		t.Fatalf("splitViewerLines() = %#v, want trailing empty line", lines)
	}
}

func TestViewerDisplayLinePadsWideRunes(t *testing.T) {
	got := viewerDisplayLine("aあ\u3000b", false)
	if got != "aあ \u3000 b" {
		t.Fatalf("viewerDisplayLine() = %q, want wide runes padded", got)
	}
}

func TestViewerDisplayLineUsesLocaleForAmbiguousRunes(t *testing.T) {
	gotNarrow := viewerDisplayLine("·", false)
	if gotNarrow != "·" {
		t.Fatalf("viewerDisplayLine(..., false) = %q, want narrow ambiguous rune", gotNarrow)
	}

	gotWide := viewerDisplayLine("·", true)
	if gotWide != "· " {
		t.Fatalf("viewerDisplayLine(..., true) = %q, want padded ambiguous rune", gotWide)
	}
}

func TestViewerLocaleUsesWideAmbiguous(t *testing.T) {
	oldLanguage := viewerSystemLanguage
	oldLocales := viewerSystemLocales
	t.Cleanup(func() {
		viewerSystemLanguage = oldLanguage
		viewerSystemLocales = oldLocales
	})

	viewerSystemLanguage = func() (string, error) { return "ja-JP", nil }
	viewerSystemLocales = func() ([]string, error) { return nil, assertError{} }
	if !viewerLocaleUsesWideAmbiguous() {
		t.Fatal("viewerLocaleUsesWideAmbiguous() = false, want true for ja-JP")
	}

	viewerSystemLanguage = func() (string, error) { return "en-US", nil }
	if viewerLocaleUsesWideAmbiguous() {
		t.Fatal("viewerLocaleUsesWideAmbiguous() = true, want false for en-US")
	}
}

func TestViewerLocaleLanguageUsesWideAmbiguous(t *testing.T) {
	tests := []struct {
		locale string
		want   bool
	}{
		{locale: "ja-JP", want: true},
		{locale: "ja_JP.UTF-8", want: true},
		{locale: "ko-KR", want: true},
		{locale: "zh-Hans-CN", want: true},
		{locale: "zh_TW", want: true},
		{locale: "en-US", want: false},
		{locale: "C", want: false},
		{locale: "POSIX", want: false},
		{locale: "", want: false},
	}

	for _, tt := range tests {
		if got := viewerLocaleLanguageUsesWideAmbiguous(tt.locale); got != tt.want {
			t.Fatalf("viewerLocaleLanguageUsesWideAmbiguous(%q) = %t, want %t", tt.locale, got, tt.want)
		}
	}
}

func TestViewerLocaleFallsBackToEnvironment(t *testing.T) {
	oldLanguage := viewerSystemLanguage
	oldLocales := viewerSystemLocales
	t.Cleanup(func() {
		viewerSystemLanguage = oldLanguage
		viewerSystemLocales = oldLocales
	})

	viewerSystemLanguage = func() (string, error) { return "", assertError{} }
	viewerSystemLocales = func() ([]string, error) { return nil, assertError{} }
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("LANG", "")

	if !viewerLocaleUsesWideAmbiguous() {
		t.Fatal("viewerLocaleUsesWideAmbiguous() = false, want environment fallback")
	}
}

type assertError struct{}

func (assertError) Error() string { return "assert error" }
