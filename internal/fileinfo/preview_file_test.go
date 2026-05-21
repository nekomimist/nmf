package fileinfo

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func TestReadPreviewFileTruncatesAtLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	data := bytes.Repeat([]byte("a"), PreviewReadLimit+10)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	preview, err := ReadPreviewFile(path)
	if err != nil {
		t.Fatalf("ReadPreviewFile returned error: %v", err)
	}
	if !preview.Truncated {
		t.Fatal("preview should be marked truncated")
	}
	if len(preview.Data) != PreviewReadLimit {
		t.Fatalf("data length = %d, want %d", len(preview.Data), PreviewReadLimit)
	}
	if len(preview.Text) != PreviewReadLimit {
		t.Fatalf("text length = %d, want %d", len(preview.Text), PreviewReadLimit)
	}
}

func TestDecodePreviewTextEncodings(t *testing.T) {
	utf8Text := strings.Repeat("日本語のテキストです。", 8)
	shiftJISText := strings.Repeat("これはShift_JISで保存された日本語のテキストです。", 8)
	shiftJIS := encodeText(t, japanese.ShiftJIS.NewEncoder(), shiftJISText)
	utf16le := append([]byte{0xff, 0xfe}, utf16Bytes("日本語", binary.LittleEndian)...)

	tests := []struct {
		name     string
		data     []byte
		wantText string
		wantEnc  string
	}{
		{name: "utf8", data: []byte(utf8Text), wantText: utf8Text, wantEnc: "UTF-8"},
		{name: "utf8 bom", data: []byte{0xef, 0xbb, 0xbf, 'h', 'i'}, wantText: "hi", wantEnc: "UTF-8"},
		{name: "shift jis", data: shiftJIS, wantText: shiftJISText, wantEnc: "Shift_JIS"},
		{name: "utf16le bom", data: utf16le, wantText: "日本語", wantEnc: "UTF-16LE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, enc := DecodePreviewText(tt.data)
			if got != tt.wantText {
				t.Fatalf("text = %q, want %q", got, tt.wantText)
			}
			if enc != tt.wantEnc {
				t.Fatalf("encoding = %q, want %q", enc, tt.wantEnc)
			}
		})
	}
}

func TestDecodePreviewTextEmpty(t *testing.T) {
	got, enc := DecodePreviewText(nil)
	if got != "" || enc != "UTF-8" {
		t.Fatalf("DecodePreviewText(nil) = %q, %q; want empty UTF-8", got, enc)
	}
}

func TestLooksBinary(t *testing.T) {
	if LooksBinary([]byte("hello\nworld")) {
		t.Fatal("plain text should not look binary")
	}
	if !LooksBinary([]byte{0x00, 0x01, 0x02, 'a'}) {
		t.Fatal("NUL/control bytes should look binary")
	}
}

func TestFormatHexDump(t *testing.T) {
	got := FormatHexDump([]byte("ABC\x00"))
	if !strings.Contains(got, "00000000") || !strings.Contains(got, "41 42 43 00") || !strings.Contains(got, "|ABC.|") {
		t.Fatalf("unexpected hex dump:\n%s", got)
	}
}

func encodeText(t *testing.T, transformer transform.Transformer, text string) []byte {
	t.Helper()
	out, err := io.ReadAll(transform.NewReader(strings.NewReader(text), transformer))
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	return out
}

func utf16Bytes(text string, order binary.ByteOrder) []byte {
	runes := utf16.Encode([]rune(text))
	out := make([]byte, len(runes)*2)
	for i, r := range runes {
		order.PutUint16(out[i*2:], r)
	}
	return out
}
