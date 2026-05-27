package fileinfo

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

const DefaultArchiveZipNameEncoding = "shift_jis"

// ArchiveOptions controls archive virtual directory behavior.
type ArchiveOptions struct {
	ZipNameEncoding string
}

var (
	archiveOptionsMu sync.RWMutex
	archiveOptions   = ArchiveOptions{ZipNameEncoding: DefaultArchiveZipNameEncoding}
)

// SetArchiveOptions sets package-wide archive options used by portable VFS helpers.
func SetArchiveOptions(opts ArchiveOptions) error {
	if _, err := ResolveArchiveZipNameEncoding(opts.ZipNameEncoding); err != nil {
		return err
	}
	if strings.TrimSpace(opts.ZipNameEncoding) == "" {
		opts.ZipNameEncoding = DefaultArchiveZipNameEncoding
	}
	archiveOptionsMu.Lock()
	defer archiveOptionsMu.Unlock()
	archiveOptions = opts
	return nil
}

func currentArchiveOptions() ArchiveOptions {
	archiveOptionsMu.RLock()
	defer archiveOptionsMu.RUnlock()
	return archiveOptions
}

// ResolveArchiveZipNameEncoding resolves the configured fallback charset for ZIP entry names.
func ResolveArchiveZipNameEncoding(name string) (encoding.Encoding, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		normalized = DefaultArchiveZipNameEncoding
	}

	switch normalized {
	case "utf-8", "utf8":
		return nil, nil
	case "shift_jis", "shift-jis", "shiftjis", "sjis", "cp932", "windows-31j", "windows31j":
		return japanese.ShiftJIS, nil
	case "cp437", "ibm437", "codepage437", "code-page-437":
		return charmap.CodePage437, nil
	}

	enc, err := ianaindex.IANA.Encoding(normalized)
	if err != nil {
		return nil, fmt.Errorf("unsupported ZIP name encoding %q", name)
	}
	if enc == nil {
		return nil, fmt.Errorf("unsupported ZIP name encoding %q", name)
	}
	return enc, nil
}

func archiveZipNameEncoding(opts ArchiveOptions) encoding.Encoding {
	enc, err := ResolveArchiveZipNameEncoding(opts.ZipNameEncoding)
	if err != nil {
		enc, _ = ResolveArchiveZipNameEncoding(DefaultArchiveZipNameEncoding)
	}
	if enc == nil {
		return nil
	}
	return utf8PreservingEncoding{fallback: enc}
}

type utf8PreservingEncoding struct {
	fallback encoding.Encoding
}

func (e utf8PreservingEncoding) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{Transformer: utf8PreservingTransformer{fallback: e.fallback.NewDecoder().Transformer}}
}

func (e utf8PreservingEncoding) NewEncoder() *encoding.Encoder {
	return e.fallback.NewEncoder()
}

type utf8PreservingTransformer struct {
	fallback transform.Transformer
}

func (t utf8PreservingTransformer) Reset() {
	t.fallback.Reset()
}

func (t utf8PreservingTransformer) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	if utf8.Valid(src) {
		if len(dst) < len(src) {
			return 0, 0, transform.ErrShortDst
		}
		copy(dst, src)
		return len(src), len(src), nil
	}
	nDst, nSrc, err = t.fallback.Transform(dst, src, atEOF)
	if err == io.EOF {
		err = nil
	}
	return nDst, nSrc, err
}
