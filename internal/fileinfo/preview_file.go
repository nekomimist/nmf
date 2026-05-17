package fileinfo

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
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
	info, err := StatPortable(p)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("viewer cannot open directories: %s", p)
	}

	vfs, parsed, err := ResolveRead(p)
	if err != nil {
		return nil, err
	}
	if closer, ok := vfs.(interface{ Close() error }); ok {
		defer closer.Close()
	}
	native := parsed.Native
	if native == "" {
		native = p
	}
	rc, err := vfs.Open(native)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, PreviewReadLimit+1))
	if err != nil {
		return nil, err
	}
	truncated := len(data) > PreviewReadLimit
	if truncated {
		data = data[:PreviewReadLimit]
	}

	text, enc := DecodePreviewText(data)
	display := parsed.Display
	if display == "" {
		display = p
	}
	return &PreviewFile{
		Path:      display,
		Name:      filepath.Base(display),
		Data:      data,
		Text:      text,
		Encoding:  enc,
		Truncated: truncated,
		Binary:    LooksBinary(data),
		Markdown:  IsMarkdownPath(display),
		Size:      info.Size(),
		SizeKnown: true,
	}, nil
}

// DecodePreviewText converts common Japanese/Unicode text encodings to UTF-8.
func DecodePreviewText(data []byte) (string, string) {
	if len(data) == 0 {
		return "", "UTF-8"
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return string(data[3:]), "UTF-8 BOM"
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
		return decodeWithEncoding(data[2:], unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), "UTF-16LE BOM")
	}
	if bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
		return decodeWithEncoding(data[2:], unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), "UTF-16BE BOM")
	}
	if utf8.Valid(data) {
		return string(data), "UTF-8"
	}

	candidates := []struct {
		name string
		enc  encoding.Encoding
	}{
		{"Shift_JIS", japanese.ShiftJIS},
		{"EUC-JP", japanese.EUCJP},
		{"UTF-16LE", unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)},
		{"UTF-16BE", unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)},
	}
	bestText := string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
	bestName := "UTF-8 replacement"
	bestScore := replacementScore(bestText)
	for _, candidate := range candidates {
		text, _ := decodeWithEncoding(data, candidate.enc, candidate.name)
		score := replacementScore(text)
		if score < bestScore {
			bestText = text
			bestName = candidate.name
			bestScore = score
		}
	}
	return bestText, bestName
}

func decodeWithEncoding(data []byte, enc encoding.Encoding, name string) (string, string) {
	reader := transform.NewReader(bytes.NewReader(data), enc.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return string(bytes.ToValidUTF8(data, []byte("\uFFFD"))), name + " replacement"
	}
	return string(decoded), name
}

func replacementScore(text string) int {
	score := strings.Count(text, "\uFFFD") * 10
	for _, r := range text {
		if r == 0 {
			score += 3
		}
	}
	return score
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
