//go:build linux
// +build linux

package fileinfo

import (
	"errors"
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

type smbReadWriteFile struct {
	file  *smb2.File
	share *smb2.Share
	sess  *smb2.Session
	conn  net.Conn
}

func (f *smbReadWriteFile) Read(p []byte) (int, error)  { return f.file.Read(p) }
func (f *smbReadWriteFile) Write(p []byte) (int, error) { return f.file.Write(p) }

func (f *smbReadWriteFile) Close() error {
	if f == nil {
		return nil
	}
	var fileErr error
	if f.file != nil {
		fileErr = f.file.Close()
		if isBenignNetworkCloseError(fileErr) {
			fileErr = nil
		}
	}
	_ = closeSMBSession(nil, f.share, f.sess, f.conn)
	return fileErr
}

type smbFileCloser struct {
	file *smb2.File
}

func (f *smbFileCloser) Read(p []byte) (int, error) {
	if f == nil || f.file == nil {
		return 0, io.EOF
	}
	return f.file.Read(p)
}

func (f *smbFileCloser) Write(p []byte) (int, error) {
	if f == nil || f.file == nil {
		return 0, os.ErrClosed
	}
	return f.file.Write(p)
}

func (f *smbFileCloser) Close() error {
	if f == nil || f.file == nil {
		return nil
	}
	err := f.file.Close()
	if isBenignNetworkCloseError(err) {
		return nil
	}
	return err
}

type smbMountedShare struct {
	share *smb2.Share
	sess  *smb2.Session
	conn  net.Conn
}

func (m *smbMountedShare) Close() error {
	if m == nil {
		return nil
	}
	return closeSMBSession(nil, m.share, m.sess, m.conn)
}

func (m *smbMountedShare) ReadDir(relPath string) ([]os.DirEntry, error) {
	p := normalizeSMBPath(relPath)
	fis, err := m.share.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]os.DirEntry, 0, len(fis))
	for _, fi := range fis {
		name := fi.Name()
		if name == "." {
			continue
		}
		out = append(out, smbDirEntry{fi: fi})
	}
	return out, nil
}

func (m *smbMountedShare) Stat(relPath string) (os.FileInfo, error) {
	return m.share.Stat(normalizeSMBPathForStat(relPath))
}

func (m *smbMountedShare) Lstat(relPath string) (os.FileInfo, error) {
	return m.share.Lstat(normalizeSMBPathForStat(relPath))
}

func (m *smbMountedShare) Open(relPath string) (io.ReadCloser, error) {
	p := normalizeSMBPath(relPath)
	if p == "" {
		return nil, fmt.Errorf("cannot open SMB share root as file")
	}
	f, err := m.share.Open(p)
	if err != nil {
		return nil, err
	}
	return &smbFileCloser{file: f}, nil
}

func (m *smbMountedShare) OpenFile(relPath string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	p := normalizeSMBPath(relPath)
	if p == "" {
		return nil, fmt.Errorf("invalid SMB file path")
	}
	f, err := m.share.OpenFile(p, flag, perm)
	if err != nil {
		return nil, err
	}
	return &smbFileCloser{file: f}, nil
}

func (m *smbMountedShare) MkdirAll(relPath string, perm os.FileMode) error {
	p := normalizeSMBPath(relPath)
	if p == "" {
		return nil
	}
	return m.share.MkdirAll(p, perm)
}

func (m *smbMountedShare) Remove(relPath string) error {
	p := normalizeSMBPath(relPath)
	if p == "" {
		return fmt.Errorf("cannot remove SMB share root")
	}
	return m.share.Remove(p)
}

func (m *smbMountedShare) Rename(oldRelPath, newRelPath string) error {
	oldp := normalizeSMBPath(oldRelPath)
	newp := normalizeSMBPath(newRelPath)
	if oldp == "" || newp == "" {
		return fmt.Errorf("invalid SMB rename path")
	}
	return m.share.Rename(oldp, newp)
}

func (m *smbMountedShare) Readlink(relPath string) (string, error) {
	p := normalizeSMBPath(relPath)
	if p == "" {
		return "", fmt.Errorf("invalid SMB symlink path")
	}
	return m.share.Readlink(p)
}

func (m *smbMountedShare) Symlink(target, linkRelPath string) error {
	linkp := normalizeSMBPath(linkRelPath)
	if linkp == "" {
		return fmt.Errorf("invalid SMB symlink path")
	}
	return m.share.Symlink(target, linkp)
}

func (m *smbMountedShare) Join(elem ...string) string {
	return "/" + strings.TrimLeft(strings.Join(elem, "/"), "/")
}

func (m *smbMountedShare) Base(p string) string {
	p = strings.TrimSuffix(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}

func (s SMBFS) credentialsFor(relPath string) Credentials {
	if s.cred != nil {
		return *s.cred
	}
	return getCredentials(s.host, s.share, relPath)
}

// OpenSession opens a reusable mounted SMB share session.
func (s SMBFS) OpenSession() (SMBSession, error) {
	share, sess, conn, _, err := s.dialAndMount("")
	if err != nil {
		return nil, err
	}
	return &smbMountedShare{share: share, sess: sess, conn: conn}, nil
}

func (s SMBFS) dialAndMount(relPath string) (*smb2.Share, *smb2.Session, net.Conn, Credentials, error) {
	creds := s.credentialsFor(relPath)
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     creds.Username,
			Password: creds.Password,
			Domain:   creds.Domain,
		},
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(s.host, "445"), 5*time.Second)
	if err != nil {
		return nil, nil, nil, creds, err
	}

	sess, err := d.Dial(conn)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		_ = conn.Close()
		return nil, nil, nil, creds, err
	}

	share, err := sess.Mount(s.share)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		_ = sess.Logoff()
		_ = conn.Close()
		return nil, nil, nil, creds, err
	}

	// Persist credentials after a successful mount if requested.
	if creds.Persist && secretStore != nil {
		_ = secretStore.Set(s.host, s.share, creds.Domain, creds.Username, creds.Password)
	}

	return share, sess, conn, creds, nil
}

func closeSMBSession(file io.Closer, share *smb2.Share, sess *smb2.Session, conn net.Conn) error {
	var firstErr error

	if file != nil {
		if err := file.Close(); err != nil && !isBenignNetworkCloseError(err) && firstErr == nil {
			firstErr = err
		}
	}
	if share != nil {
		if err := share.Umount(); err != nil && !isBenignNetworkCloseError(err) && firstErr == nil {
			firstErr = err
		}
	}
	if sess != nil {
		if err := sess.Logoff(); err != nil && !isBenignNetworkCloseError(err) && firstErr == nil {
			firstErr = err
		}
	}
	if conn != nil {
		if err := conn.Close(); err != nil && !isBenignNetworkCloseError(err) && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func isBenignNetworkCloseError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	// Keep a string fallback for wrapped/translated runtime errors.
	return strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

func normalizeSMBPath(relPath string) string {
	p := strings.TrimSpace(relPath)
	if p == "/" || p == "\\" {
		return ""
	}
	p = strings.ReplaceAll(p, "\\", "/")
	for strings.HasPrefix(p, "/") {
		p = p[1:]
	}
	return p
}

func normalizeSMBPathForStat(relPath string) string {
	p := normalizeSMBPath(relPath)
	if p == "" {
		return "."
	}
	return p
}

func (s SMBFS) ReadDir(relPath string) ([]os.DirEntry, error) {
	share, sess, conn, _, err := s.dialAndMount(relPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = closeSMBSession(nil, share, sess, conn)
	}()

	p := normalizeSMBPath(relPath)
	fis, err := share.ReadDir(p)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		return nil, err
	}
	out := make([]os.DirEntry, 0, len(fis))
	for _, fi := range fis {
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
	share, sess, conn, _, err := s.dialAndMount(relPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = closeSMBSession(nil, share, sess, conn)
	}()

	fi, err := share.Stat(normalizeSMBPathForStat(relPath))
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		return nil, err
	}
	return fi, nil
}

// StorageInfo returns capacity information for the SMB share.
func (s SMBFS) StorageInfo(relPath string) (StorageInfo, error) {
	share, sess, conn, _, err := s.dialAndMount(relPath)
	if err != nil {
		return StorageInfo{}, err
	}
	defer func() {
		_ = closeSMBSession(nil, share, sess, conn)
	}()

	info, err := share.Statfs(normalizeSMBPathForStat(relPath))
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		return StorageInfo{}, err
	}
	blockSize := info.BlockSize() * info.FragmentSize()
	return storageInfoFromBlocks(info.TotalBlockCount(), info.AvailableBlockCount(), blockSize), nil
}

// Lstat returns file info without following symlinks.
func (s SMBFS) Lstat(relPath string) (os.FileInfo, error) {
	share, sess, conn, _, err := s.dialAndMount(relPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = closeSMBSession(nil, share, sess, conn)
	}()

	fi, err := share.Lstat(normalizeSMBPathForStat(relPath))
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

// Open opens a file for reading.
func (s SMBFS) Open(relPath string) (io.ReadCloser, error) {
	share, sess, conn, _, err := s.dialAndMount(relPath)
	if err != nil {
		return nil, err
	}

	p := normalizeSMBPath(relPath)
	if p == "" {
		_ = closeSMBSession(nil, share, sess, conn)
		return nil, fmt.Errorf("cannot open SMB share root as file")
	}

	f, err := share.Open(p)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		_ = closeSMBSession(nil, share, sess, conn)
		return nil, err
	}

	return &smbReadWriteFile{file: f, share: share, sess: sess, conn: conn}, nil
}

// OpenFile opens a file with flags for read/write operations.
func (s SMBFS) OpenFile(relPath string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	share, sess, conn, _, err := s.dialAndMount(relPath)
	if err != nil {
		return nil, err
	}

	p := normalizeSMBPath(relPath)
	if p == "" {
		_ = closeSMBSession(nil, share, sess, conn)
		return nil, fmt.Errorf("invalid SMB file path")
	}

	f, err := share.OpenFile(p, flag, perm)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		_ = closeSMBSession(nil, share, sess, conn)
		return nil, err
	}

	return &smbReadWriteFile{file: f, share: share, sess: sess, conn: conn}, nil
}

// MkdirAll creates a directory path (including parents).
func (s SMBFS) MkdirAll(relPath string, perm os.FileMode) error {
	p := normalizeSMBPath(relPath)
	if p == "" {
		return nil
	}

	share, sess, conn, _, err := s.dialAndMount(relPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = closeSMBSession(nil, share, sess, conn)
	}()

	err = share.MkdirAll(p, perm)
	if err != nil && isAuthError(err) {
		ClearCachedCredentials(s.host, s.share)
	}
	return err
}

// Remove removes a file or an empty directory.
func (s SMBFS) Remove(relPath string) error {
	p := normalizeSMBPath(relPath)
	if p == "" {
		return fmt.Errorf("cannot remove SMB share root")
	}

	share, sess, conn, _, err := s.dialAndMount(relPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = closeSMBSession(nil, share, sess, conn)
	}()

	err = share.Remove(p)
	if err != nil && isAuthError(err) {
		ClearCachedCredentials(s.host, s.share)
	}
	return err
}

// Rename renames a file or directory path within the same share.
func (s SMBFS) Rename(oldRelPath, newRelPath string) error {
	oldp := normalizeSMBPath(oldRelPath)
	newp := normalizeSMBPath(newRelPath)
	if oldp == "" || newp == "" {
		return fmt.Errorf("invalid SMB rename path")
	}

	share, sess, conn, _, err := s.dialAndMount(oldRelPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = closeSMBSession(nil, share, sess, conn)
	}()

	err = share.Rename(oldp, newp)
	if err != nil && isAuthError(err) {
		ClearCachedCredentials(s.host, s.share)
	}
	return err
}

// Readlink reads symlink target path.
func (s SMBFS) Readlink(relPath string) (string, error) {
	p := normalizeSMBPath(relPath)
	if p == "" {
		return "", fmt.Errorf("invalid SMB symlink path")
	}

	share, sess, conn, _, err := s.dialAndMount(relPath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = closeSMBSession(nil, share, sess, conn)
	}()

	target, err := share.Readlink(p)
	if err != nil {
		if isAuthError(err) {
			ClearCachedCredentials(s.host, s.share)
		}
		return "", err
	}
	return target, nil
}

// Symlink creates a symlink at linkRelPath with target.
func (s SMBFS) Symlink(target, linkRelPath string) error {
	linkp := normalizeSMBPath(linkRelPath)
	if linkp == "" {
		return fmt.Errorf("invalid SMB symlink path")
	}

	share, sess, conn, _, err := s.dialAndMount(linkRelPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = closeSMBSession(nil, share, sess, conn)
	}()

	err = share.Symlink(target, linkp)
	if err != nil && isAuthError(err) {
		ClearCachedCredentials(s.host, s.share)
	}
	return err
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
