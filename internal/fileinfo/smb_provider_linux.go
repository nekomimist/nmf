//go:build linux
// +build linux

package fileinfo

func newSMBProvider(host, share string, c *Credentials) VFS {
	if c != nil {
		return NewSMBFSWithCred(host, share, *c)
	}
	return NewSMBFS(host, share)
}
