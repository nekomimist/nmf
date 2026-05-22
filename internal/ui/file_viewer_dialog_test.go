package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2/test"

	"nmf/internal/fileinfo"
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
