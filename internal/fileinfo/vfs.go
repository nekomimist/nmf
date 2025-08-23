package fileinfo

import (
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

// LocalFS implements VFS using the host OS.
type LocalFS struct{}

func (LocalFS) ReadDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }
func (LocalFS) Stat(path string) (os.FileInfo, error)      { return os.Stat(path) }
func (LocalFS) Capabilities() Capabilities                 { return Capabilities{FastList: true, Watch: true} }
func (LocalFS) Join(elem ...string) string                 { return filepath.Join(elem...) }
func (LocalFS) Base(p string) string                       { return filepath.Base(p) }
func (LocalFS) Open(path string) (io.ReadCloser, error)    { return os.Open(path) }
