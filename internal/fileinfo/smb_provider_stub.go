//go:build !linux
// +build !linux

package fileinfo

// On non-Linux platforms, SMB direct provider is not available; fall back to LocalFS.
func newSMBProvider(host, share string, c *Credentials) VFS {
	return LocalFS{}
}
