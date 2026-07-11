package fileinfo

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogs/chardet"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
)

const PreviewReadLimit = 1 << 20

// PreviewFile contains the bounded content used by the built-in viewer.
type PreviewFile struct {
	Path      string
	Name      string
	Data      []byte
	Text      string
	Encoding  string
	Truncated bool
	Binary    bool
	Markdown  bool
	Size      int64
	SizeKnown bool
}

// ReadPreviewFile reads at most PreviewReadLimit bytes from p and prepares text
// and binary views for the built-in viewer.
func ReadPreviewFile(p string) (*PreviewFile, error) {
	return ReadPreviewFileWithDebug(p, nil)
}

// ReadPreviewFileWithDebug is ReadPreviewFile with optional timing logs.
func ReadPreviewFileWithDebug(p string, debugPrint func(format string, args ...interface{})) (*PreviewFile, error) {
	totalStart := time.Now()
	stepStart := totalStart
	info, err := StatPortable(p)
	previewDebug(debugPrint, "FileViewer: stat elapsed=%s path=%s err=%v", time.Since(stepStart), p, err)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("viewer cannot open directories: %s", p)
	}

	stepStart = time.Now()
	vfs, parsed, err := ResolveRead(p)
	previewDebug(debugPrint, "FileViewer: resolve elapsed=%s path=%s err=%v", time.Since(stepStart), p, err)
	if err != nil {
		return nil, err
	}
	defer CloseVFS(vfs)
	native := parsed.Native
	if native == "" {
		native = p
	}
	stepStart = time.Now()
	rc, err := vfs.Open(native)
	previewDebug(debugPrint, "FileViewer: open elapsed=%s native=%s err=%v", time.Since(stepStart), native, err)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	stepStart = time.Now()
	data, err := io.ReadAll(io.LimitReader(rc, PreviewReadLimit+1))
	previewDebug(debugPrint, "FileViewer: read elapsed=%s bytes=%d err=%v", time.Since(stepStart), len(data), err)
	if err != nil {
		return nil, err
	}
	truncated := len(data) > PreviewReadLimit
	if truncated {
		data = data[:PreviewReadLimit]
	}

	stepStart = time.Now()
	text, enc := DecodePreviewText(data)
	previewDebug(debugPrint, "FileViewer: decode elapsed=%s bytes=%d text_bytes=%d encoding=%s", time.Since(stepStart), len(data), len(text), enc)
	stepStart = time.Now()
	binary := LooksBinary(data)
	previewDebug(debugPrint, "FileViewer: binary-check elapsed=%s binary=%t", time.Since(stepStart), binary)
	display := parsed.Display
	if display == "" {
		display = p
	}
	preview := &PreviewFile{
		Path:      display,
		Name:      filepath.Base(display),
		Data:      data,
		Text:      text,
		Encoding:  enc,
		Truncated: truncated,
		Binary:    binary,
		Markdown:  IsMarkdownPath(display),
		Size:      info.Size(),
		SizeKnown: true,
	}
	previewDebug(debugPrint, "FileViewer: preview-ready elapsed=%s bytes=%d text_bytes=%d binary=%t truncated=%t",
		time.Since(totalStart), len(preview.Data), len(preview.Text), preview.Binary, preview.Truncated)
	return preview, nil
}

func previewDebug(debugPrint func(format string, args ...interface{}), format string, args ...interface{}) {
	if debugPrint != nil {
		debugPrint(format, args...)
	}
}

// DecodePreviewText detects text encoding and converts it to valid UTF-8.
func DecodePreviewText(data []byte) (string, string) {
	if len(data) == 0 {
		return "", "UTF-8"
	}
	result, err := chardet.NewTextDetector().DetectBest(data)
	if err != nil || result == nil || result.Charset == "" {
		return replacementText(data), "UTF-8 replacement"
	}
	enc, err := ianaindex.IANA.Encoding(result.Charset)
	if err != nil || enc == nil {
		return replacementText(data), result.Charset + " replacement"
	}
	reader := transform.NewReader(bytes.NewReader(data), enc.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return replacementText(data), result.Charset + " replacement"
	}
	return strings.TrimPrefix(replacementText(decoded), "\uFEFF"), result.Charset
}

func replacementText(data []byte) string {
	return string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
}

// LooksBinary reports whether data appears to be binary rather than text.
func LooksBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	nul := 0
	control := 0
	for _, b := range data {
		switch {
		case b == 0:
			nul++
		case b < 0x09 || (b > 0x0D && b < 0x20):
			control++
		}
	}
	return nul > 0 || control*100/len(data) > 5
}

// IsMarkdownPath reports whether p has a Markdown-like extension.
func IsMarkdownPath(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".markdown", ".mdown":
		return true
	default:
		return false
	}
}

// FormatHexDump returns a classic offset/hex/ascii dump of data.
func FormatHexDump(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var b strings.Builder
	for offset := 0; offset < len(data); offset += 16 {
		line := data[offset:]
		if len(line) > 16 {
			line = line[:16]
		}
		fmt.Fprintf(&b, "%08x  ", offset)
		for i := 0; i < 16; i++ {
			if i < len(line) {
				fmt.Fprintf(&b, "%02x ", line[i])
			} else {
				b.WriteString("   ")
			}
			if i == 7 {
				b.WriteByte(' ')
			}
		}
		b.WriteString(" |")
		for _, c := range line {
			if c >= 0x20 && c <= 0x7e {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("|\n")
	}
	return b.String()
}
