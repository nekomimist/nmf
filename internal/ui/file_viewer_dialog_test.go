package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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
	for _, want := range []string{`hello\x1b[31m`, "\t日本語\n", `zero-width:\u200b\x7f`} {
		if !strings.Contains(got, want) {
			t.Fatalf("viewerText() = %q, want fragment %q", got, want)
		}
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
