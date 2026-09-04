package fileinfo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/gen2brain/jxl"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
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

func TestReadPreviewFileInsideBangDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bang!")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dir, "readme.txt")
	const content = "ordinary directory, not an archive\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	preview, err := ReadPreviewFile(filePath)
	if err != nil {
		t.Fatalf("ReadPreviewFile(%q) returned error: %v", filePath, err)
	}
	if preview.Text != content {
		t.Fatalf("preview text = %q, want %q", preview.Text, content)
	}
}

func TestReadPreviewFileDecodesSupportedImagesByContent(t *testing.T) {
	dir := t.TempDir()
	source := image.NewNRGBA(image.Rect(0, 0, 3, 2))

	tests := []struct {
		name   string
		format string
		encode func(io.Writer, image.Image) error
	}{
		{name: "png with wrong extension", format: "PNG", encode: png.Encode},
		{name: "jpeg", format: "JPEG", encode: func(w io.Writer, img image.Image) error { return jpeg.Encode(w, img, nil) }},
		{name: "jpeg xl", format: "JPEG XL", encode: func(w io.Writer, img image.Image) error {
			return jxl.Encode(w, img, jxl.EncodeOptions{Lossless: true, Threads: 1})
		}},
		{name: "gif", format: "GIF", encode: func(w io.Writer, img image.Image) error { return gif.Encode(w, img, nil) }},
		{name: "bmp", format: "BMP", encode: bmp.Encode},
		{name: "tiff", format: "TIFF", encode: func(w io.Writer, img image.Image) error { return tiff.Encode(w, img, nil) }},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var encoded bytes.Buffer
			if err := tt.encode(&encoded, source); err != nil {
				t.Fatalf("encode failed: %v", err)
			}
			path := filepath.Join(dir, fmt.Sprintf("image-%d.dat", i))
			if err := os.WriteFile(path, encoded.Bytes(), 0644); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			preview, err := ReadPreviewFile(path)
			if err != nil {
				t.Fatalf("ReadPreviewFile returned error: %v", err)
			}
			if preview.Image == nil || preview.ImageFormat != tt.format {
				t.Fatalf("image=%v format=%q, want decoded %s", preview.Image != nil, preview.ImageFormat, tt.format)
			}
			if preview.ImageWidth != 3 || preview.ImageHeight != 2 || !preview.Binary {
				t.Fatalf("image metadata = %dx%d binary=%t", preview.ImageWidth, preview.ImageHeight, preview.Binary)
			}
		})
	}
}

func TestLooksLikeJPEGXL(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "codestream", data: []byte{'\xff', '\x0a'}, want: true},
		{name: "container", data: []byte("\x00\x00\x00\x0cJXL \x0d\x0a\x87\x0a"), want: true},
		{name: "jpeg", data: []byte{'\xff', '\xd8', '\xff'}, want: false},
		{name: "truncated container", data: []byte("\x00\x00\x00\x0cJXL "), want: false},
		{name: "empty", data: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeJPEGXL(tt.data); got != tt.want {
				t.Fatalf("looksLikeJPEGXL() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestReadPreviewFileDecodesWebP(t *testing.T) {
	const webPBase64 = "UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA=="
	data, err := base64.StdEncoding.DecodeString(webPBase64)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "image.bin")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	preview, err := ReadPreviewFile(path)
	if err != nil {
		t.Fatalf("ReadPreviewFile returned error: %v", err)
	}
	if preview.Image == nil || preview.ImageFormat != "WebP" {
		t.Fatalf("image=%v format=%q, want decoded WebP", preview.Image != nil, preview.ImageFormat)
	}
}

func TestReadPreviewFileStreamsImageBeyondHexLimit(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	for i := range source.Pix {
		source.Pix[i] = byte(i * 31)
	}
	var encoded bytes.Buffer
	if err := bmp.Encode(&encoded, source); err != nil {
		t.Fatalf("bmp.Encode failed: %v", err)
	}
	if encoded.Len() <= PreviewReadLimit {
		t.Fatalf("encoded BMP size = %d, want over preview limit", encoded.Len())
	}
	path := filepath.Join(t.TempDir(), "large.bmp")
	if err := os.WriteFile(path, encoded.Bytes(), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	preview, err := ReadPreviewFile(path)
	if err != nil {
		t.Fatalf("ReadPreviewFile returned error: %v", err)
	}
	if preview.Image == nil || len(preview.Data) != PreviewReadLimit || !preview.Truncated {
		t.Fatalf("decoded=%t data=%d truncated=%t", preview.Image != nil, len(preview.Data), preview.Truncated)
	}
}

func TestReadPreviewFileCorruptImageFallsBackWithoutFatalError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.dat")
	data := append([]byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n'}, []byte("broken")...)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	preview, err := ReadPreviewFile(path)
	if err != nil {
		t.Fatalf("ReadPreviewFile returned error: %v", err)
	}
	if preview.ImageFormat != "PNG" || preview.Image != nil || preview.ImageError == "" {
		t.Fatalf("format=%q image=%v error=%q", preview.ImageFormat, preview.Image != nil, preview.ImageError)
	}
}

func TestReadPreviewFileContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadPreviewFileContext(ctx, "unused"); err == nil {
		t.Fatal("ReadPreviewFileContext should return cancellation")
	}
}

func TestValidatePreviewImageDimensions(t *testing.T) {
	for _, tt := range []struct {
		width, height int
		wantErr       bool
	}{
		{width: 8000, height: 8000},
		{width: 8001, height: 8000, wantErr: true},
		{width: ImagePreviewEdgeLimit + 1, height: 1, wantErr: true},
		{width: 0, height: 1, wantErr: true},
	} {
		if got := validatePreviewImageDimensions(tt.width, tt.height); (got != nil) != tt.wantErr {
			t.Fatalf("validatePreviewImageDimensions(%d, %d) = %v, wantErr=%t", tt.width, tt.height, got, tt.wantErr)
		}
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

// Text files whose first two bytes happen to be "BM" must not be classified as
// images: an image classification skips text decoding entirely, which would
// leave the file with no readable pane in the viewer.
func TestReadPreviewFileKeepsTextStartingWithBM(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		content string
	}{
		{name: "bmw.csv", content: "BMW parts list\nfront brake,12000\nrear brake,9800\n"},
		{name: "notes.md", content: "BM (Business Model) memo\n\n- item one\n- item two\n"},
		{name: "just-bm.txt", content: "BM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name)
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			preview, err := ReadPreviewFile(path)
			if err != nil {
				t.Fatalf("ReadPreviewFile returned error: %v", err)
			}
			if preview.ImageFormat != "" {
				t.Fatalf("ImageFormat = %q, want empty", preview.ImageFormat)
			}
			if preview.Binary {
				t.Fatal("Binary = true, want false")
			}
			if preview.Text != tt.content {
				t.Fatalf("Text = %q, want %q", preview.Text, tt.content)
			}
		})
	}
}

func TestLooksLikeBMP(t *testing.T) {
	header := func(infoLen uint32) []byte {
		b := make([]byte, 18)
		copy(b, "BM")
		binary.LittleEndian.PutUint32(b[14:18], infoLen)
		return b
	}
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "BITMAPINFOHEADER", data: header(40), want: true},
		{name: "BITMAPV4HEADER", data: header(108), want: true},
		{name: "BITMAPV5HEADER", data: header(124), want: true},
		{name: "BITMAPCOREHEADER is not decodable", data: header(12), want: false},
		{name: "text starting with BM", data: []byte("BMW parts list\nfront brake"), want: false},
		{name: "too short", data: []byte("BM"), want: false},
		{name: "wrong signature", data: header(40)[1:], want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeBMP(tt.data); got != tt.want {
				t.Fatalf("looksLikeBMP() = %t, want %t", got, tt.want)
			}
		})
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
