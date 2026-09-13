package fileinfo

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Capabilities describes provider abilities (reserved for future use).
type Capabilities struct {
	FastList bool
	Watch    bool
}

// VFS defines minimal operations nmf needs for listing/metadata.
// Initial rollout used ReadDir only; we now add Stat and basic path helpers.
type VFS interface {
	ReadDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	Capabilities() Capabilities
	// Basic helpers so callers can avoid filepath on virtual paths
	Join(elem ...string) string
	Base(p string) string
	// Optional: Open for previews; providers may return an error if unsupported
	Open(path string) (io.ReadCloser, error)
}

// CloseVFS releases resources owned by a resolved provider. Most providers are
// stateless, but a remote ArchiveVFS owns a downloaded temporary source.
func CloseVFS(vfs VFS) error {
	if closer, ok := vfs.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

type resolvedReadCloser struct {
	io.ReadCloser
	vfs VFS
}

func (r *resolvedReadCloser) Close() error {
	if r == nil {
		return nil
	}
	var readErr error
	if r.ReadCloser != nil {
		readErr = r.ReadCloser.Close()
		r.ReadCloser = nil
	}
	vfsErr := CloseVFS(r.vfs)
	r.vfs = nil
	return errors.Join(readErr, vfsErr)
}

// OpenPortable resolves and opens p. Closing the returned reader also closes
// the resolved VFS, including any remote archive source temporary file.
func OpenPortable(p string) (io.ReadCloser, error) {
	return OpenPortableContext(context.Background(), p)
}

// OpenPortableContext propagates cancellation to providers that support it.
func OpenPortableContext(ctx context.Context, p string) (io.ReadCloser, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	vfs, parsed, err := ResolveReadContext(ctx, p)
	if err != nil {
		return nil, err
	}
	native := parsed.Native
	if native == "" {
		native = p
	}
	var reader io.ReadCloser
	if provider, ok := vfs.(interface {
		OpenContext(context.Context, string) (io.ReadCloser, error)
	}); ok {
		reader, err = provider.OpenContext(ctx, native)
	} else {
		reader, err = vfs.Open(native)
	}
	if err != nil {
		_ = CloseVFS(vfs)
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		reader.Close()
		CloseVFS(vfs)
		return nil, err
	}
	return &resolvedReadCloser{ReadCloser: reader, vfs: vfs}, nil
}

// LocalFS implements VFS using the host OS.
type LocalFS struct{}

func (LocalFS) ReadDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }
func (LocalFS) Stat(path string) (os.FileInfo, error)      { return os.Stat(path) }
func (LocalFS) Lstat(path string) (os.FileInfo, error)     { return os.Lstat(path) }
func (LocalFS) Readlink(path string) (string, error)       { return os.Readlink(path) }
func (LocalFS) Capabilities() Capabilities                 { return Capabilities{FastList: true, Watch: true} }
func (LocalFS) Join(elem ...string) string                 { return filepath.Join(elem...) }
func (LocalFS) Base(p string) string                       { return filepath.Base(p) }
func (LocalFS) Open(path string) (io.ReadCloser, error)    { return os.Open(path) }
