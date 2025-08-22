package fileinfo

import (
	"os"
)

// Capabilities describes provider abilities (reserved for future use).
type Capabilities struct {
	FastList bool
	Watch    bool
}

// VFS defines minimal operations nmf needs for listing/metadata.
// Initial rollout only uses ReadDir via LocalFS; SMB providers will be added later.
type VFS interface {
	ReadDir(path string) ([]os.DirEntry, error)
	Capabilities() Capabilities
}

// LocalFS implements VFS using the host OS.
type LocalFS struct{}

func (LocalFS) ReadDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }
func (LocalFS) Capabilities() Capabilities                 { return Capabilities{FastList: true, Watch: true} }
