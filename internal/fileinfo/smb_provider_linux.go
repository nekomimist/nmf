//go:build linux
// +build linux

package fileinfo

func newSMBProvider(host, share string, c *Credentials) (VFS, error) {
	if c != nil {
		return NewSMBFSWithCred(host, share, *c), nil
	}
	return NewSMBFS(host, share), nil
}
