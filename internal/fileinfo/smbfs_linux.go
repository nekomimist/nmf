//go:build linux
// +build linux

package fileinfo

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// SMBFS implements VFS for direct SMB access on Linux.
type SMBFS struct {
	host  string
	share string
	cred  *Credentials
}

func NewSMBFS(host, share string) SMBFS { return SMBFS{host: host, share: share} }
func NewSMBFSWithCred(host, share string, c Credentials) SMBFS {
	return SMBFS{host: host, share: share, cred: &c}
}

func (SMBFS) Capabilities() Capabilities { return Capabilities{FastList: false, Watch: false} }

func (s SMBFS) ReadDir(relPath string) ([]os.DirEntry, error) {
	creds := Credentials{}
	if s.cred != nil {
		creds = *s.cred
	} else {
		creds = getCredentials(s.host, s.share, relPath)
	}

	// Establish TCP connection to SMB port
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     creds.Username,
			Password: creds.Password,
			Domain:   creds.Domain,
		},
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(s.host, "445"), 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	sess, err := d.Dial(conn)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		return nil, err
	}
	defer sess.Logoff()

	share, err := sess.Mount(s.share)
	if err != nil {
		return nil, err
	}
	defer share.Umount()

	// Persist credentials after a successful mount if requested
	if creds.Persist && secretStore != nil {
		_ = secretStore.Set(s.host, s.share, creds.Domain, creds.Username, creds.Password)
	}

	// normalize relPath relative to the share; go-smb2 forbids leading '\\'
	// Acceptable values: "" (root), or relative like "a/b".
	p := relPath
	if p == "/" || p == "\\" {
		p = ""
	}
	// Strip any leading separators
	for len(p) > 0 && (p[0] == '/' || p[0] == '\\') {
		p = p[1:]
	}
	// Read dir entries via SMB
	fis, err := share.ReadDir(p)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		return nil, err
	}
	out := make([]os.DirEntry, 0, len(fis))
	for _, fi := range fis {
		// skip "." entries if any
		name := fi.Name()
		if name == "." {
			continue
		}
		out = append(out, smbDirEntry{fi: fi})
	}
	return out, nil
}

// Stat returns file info for a path relative to the share (leading separators allowed).
func (s SMBFS) Stat(relPath string) (os.FileInfo, error) {
	creds := Credentials{}
	if s.cred != nil {
		creds = *s.cred
	} else {
		creds = getCredentials(s.host, s.share, relPath)
	}

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     creds.Username,
			Password: creds.Password,
			Domain:   creds.Domain,
		},
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(s.host, "445"), 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	sess, err := d.Dial(conn)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		return nil, err
	}
	defer sess.Logoff()

	share, err := sess.Mount(s.share)
	if err != nil {
		return nil, err
	}
	defer share.Umount()

	if creds.Persist && secretStore != nil {
		_ = secretStore.Set(s.host, s.share, creds.Domain, creds.Username, creds.Password)
	}

	p := relPath
	if p == "/" || p == "\\" {
		p = ""
	}
	for len(p) > 0 && (p[0] == '/' || p[0] == '\\') {
		p = p[1:]
	}
	// When empty => share root, Stat on "." (go-smb2 requires non-empty?)
	if p == "" {
		p = "."
	}
	fi, err := share.Stat(p)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		return nil, err
	}
	return fi, nil
}

// Join joins relative path elements using forward slashes (provider-native for SMBFS).
func (SMBFS) Join(elem ...string) string { return "/" + strings.TrimLeft(strings.Join(elem, "/"), "/") }

// Base returns last element after splitting by '/'.
func (SMBFS) Base(p string) string {
	p = strings.TrimSuffix(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}

// Open is not implemented for SMBFS in this phase.
func (SMBFS) Open(path string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("Open not implemented for SMBFS")
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	e := strings.ToLower(err.Error())
	// Common indicators from Windows/SMB servers
	if strings.Contains(e, "logon is invalid") ||
		strings.Contains(e, "bad username") ||
		strings.Contains(e, "authentication") ||
		strings.Contains(e, "status_logon_failure") ||
		strings.Contains(e, "access is denied") {
		return true
	}
	return false
}
