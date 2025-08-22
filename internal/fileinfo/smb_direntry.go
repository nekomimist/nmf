//go:build linux
// +build linux

package fileinfo

import "os"

// smbDirEntry adapts os.FileInfo to os.DirEntry for list rendering.
type smbDirEntry struct{ fi os.FileInfo }

func (e smbDirEntry) Name() string               { return e.fi.Name() }
func (e smbDirEntry) IsDir() bool                { return e.fi.IsDir() }
func (e smbDirEntry) Type() os.FileMode          { return e.fi.Mode().Type() }
func (e smbDirEntry) Info() (os.FileInfo, error) { return e.fi, nil }
