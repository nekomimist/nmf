package fileinfo

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogs/chardet"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
)

const (
	PreviewReadLimit       = 1 << 20
	ImagePreviewPixelLimit = 64_000_000
	ImagePreviewEdgeLimit  = 32_768
)

// PreviewFile contains the bounded content used by the built-in viewer.
type PreviewFile struct {
	Path        string
	Name        string
	Data        []byte
	Text        string
	Encoding    string
	Truncated   bool
	Binary      bool
	Markdown    bool
	Size        int64
	SizeKnown   bool
	Image       image.Image
	ImageFormat string
	ImageWidth  int
	ImageHeight int
	ImageError  string
}

// ReadPreviewFile prepares image, text, and binary views for the built-in
// viewer. Text and binary data are bounded by PreviewReadLimit; recognized
// images may continue decoding from the source stream.
func ReadPreviewFile(p string) (*PreviewFile, error) {
	return ReadPreviewFileWithDebugContext(context.Background(), p, nil)
}

// ReadPreviewFileWithDebug is ReadPreviewFile with optional timing logs.
func ReadPreviewFileWithDebug(p string, debugPrint func(format string, args ...interface{})) (*PreviewFile, error) {
	return ReadPreviewFileWithDebugContext(context.Background(), p, debugPrint)
}

// ReadPreviewFileContext is ReadPreviewFile with best-effort cancellation.
func ReadPreviewFileContext(ctx context.Context, p string) (*PreviewFile, error) {
	return ReadPreviewFileWithDebugContext(ctx, p, nil)
}

// ReadPreviewFileWithDebugContext reads a preview using ctx and optional timing logs.
func ReadPreviewFileWithDebugContext(ctx context.Context, p string, debugPrint func(format string, args ...interface{})) (*PreviewFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	totalStart := time.Now()
	stepStart := totalStart
	vfs, parsed, err := ResolveReadContext(ctx, p)
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
	info, err := vfs.Stat(native)
	previewDebug(debugPrint, "FileViewer: stat elapsed=%s path=%s err=%v", time.Since(stepStart), p, err)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("viewer cannot open directories: %s", p)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stepStart = time.Now()
	rc, err := vfs.Open(native)
	previewDebug(debugPrint, "FileViewer: open elapsed=%s native=%s err=%v", time.Since(stepStart), native, err)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	stepStart = time.Now()
	rawPrefix, err := io.ReadAll(io.LimitReader(&previewContextReader{ctx: ctx, reader: rc}, PreviewReadLimit+1))
	previewDebug(debugPrint, "FileViewer: read elapsed=%s bytes=%d err=%v", time.Since(stepStart), len(rawPrefix), err)
	if err != nil {
		return nil, err
	}
	truncated := len(rawPrefix) > PreviewReadLimit
	data := rawPrefix
	if len(data) > PreviewReadLimit {
		data = data[:PreviewReadLimit]
	}

	display := parsed.Display
	if display == "" {
		display = p
	}
	preview := &PreviewFile{
		Path:      display,
		Name:      filepath.Base(display),
		Data:      data,
		Truncated: truncated,
		Markdown:  IsMarkdownPath(display),
		Size:      info.Size(),
		SizeKnown: true,
	}

	if format := supportedPreviewImageFormat(rawPrefix); format != "" {
		preview.Binary = true
		preview.ImageFormat = format
		stepStart = time.Now()
		cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(rawPrefix))
		previewDebug(debugPrint, "FileViewer: image-config elapsed=%s format=%s width=%d height=%d err=%v",
			time.Since(stepStart), format, cfg.Width, cfg.Height, cfgErr)
		if cfgErr != nil {
			preview.ImageError = fmt.Sprintf("invalid %s image", format)
		} else {
			preview.ImageWidth = cfg.Width
			preview.ImageHeight = cfg.Height
			if err := validatePreviewImageDimensions(cfg.Width, cfg.Height); err != nil {
				preview.ImageError = err.Error()
			} else {
				stepStart = time.Now()
				decoded, _, decodeErr := image.Decode(io.MultiReader(
					bytes.NewReader(rawPrefix),
					&previewContextReader{ctx: ctx, reader: rc},
				))
				previewDebug(debugPrint, "FileViewer: image-decode elapsed=%s format=%s err=%v", time.Since(stepStart), format, decodeErr)
				if decodeErr != nil {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					preview.ImageError = fmt.Sprintf("failed to decode %s image", format)
				} else {
					preview.Image = decoded
				}
			}
		}
	} else {
		stepStart = time.Now()
		preview.Text, preview.Encoding = DecodePreviewText(data)
		previewDebug(debugPrint, "FileViewer: decode elapsed=%s bytes=%d text_bytes=%d encoding=%s",
			time.Since(stepStart), len(data), len(preview.Text), preview.Encoding)
		stepStart = time.Now()
		preview.Binary = LooksBinary(data)
		previewDebug(debugPrint, "FileViewer: binary-check elapsed=%s binary=%t", time.Since(stepStart), preview.Binary)
	}
	previewDebug(debugPrint, "FileViewer: preview-ready elapsed=%s bytes=%d text_bytes=%d binary=%t truncated=%t",
		time.Since(totalStart), len(preview.Data), len(preview.Text), preview.Binary, preview.Truncated)
	return preview, nil
}

type previewContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *previewContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if err == nil {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			return n, ctxErr
		}
	}
	return n, err
}

func validatePreviewImageDimensions(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("image has invalid dimensions %dx%d", width, height)
	}
	if width > ImagePreviewEdgeLimit || height > ImagePreviewEdgeLimit || int64(width)*int64(height) > ImagePreviewPixelLimit {
		return fmt.Errorf("image too large: %dx%d (limit %d pixels, edge %d)",
			width, height, ImagePreviewPixelLimit, ImagePreviewEdgeLimit)
	}
	return nil
}

func supportedPreviewImageFormat(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n'}):
		return "PNG"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "JPEG"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return "GIF"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "WebP"
	case len(data) >= 2 && bytes.Equal(data[:2], []byte("BM")):
		return "BMP"
	case len(data) >= 4 && (bytes.Equal(data[:4], []byte{'I', 'I', '*', 0}) || bytes.Equal(data[:4], []byte{'M', 'M', 0, '*'})):
		return "TIFF"
	default:
		return ""
	}
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
