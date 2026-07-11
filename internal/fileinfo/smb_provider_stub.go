//go:build !linux
// +build !linux

package fileinfo

// On non-Linux platforms, direct SMB is unavailable. Windows resolves SMB URLs
// through its native UNC branch before reaching this function; other platforms
// must fail explicitly instead of treating SMB-relative paths as local paths.
func newSMBProvider(host, share string, c *Credentials) (VFS, error) {
	return nil, errUnsupportedSMB()
}
