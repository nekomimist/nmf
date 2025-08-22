//go:build windows
// +build windows

package fileinfo

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// Windows API constants
const (
	RESOURCETYPE_DISK                 = 0x00000001
	CONNECT_TEMPORARY                 = 0x00000004
	NO_ERROR                          = 0
	ERROR_ACCESS_DENIED               = 5
	ERROR_LOGON_FAILURE               = 1326
	ERROR_SESSION_CREDENTIAL_CONFLICT = 1219
)

type netResource struct {
	DwScope       uint32
	DwType        uint32
	DwDisplayType uint32
	DwUsage       uint32
	LpLocalName   *uint16
	LpRemoteName  *uint16
	LpComment     *uint16
	LpProvider    *uint16
}

var (
	modMpr                  = syscall.NewLazyDLL("mpr.dll")
	procWNetAddConnection2W = modMpr.NewProc("WNetAddConnection2W")
)

func addConnection(host, share, username, password string) (uint32, error) {
	remote := `\\` + host + `\` + share
	remotePtr, _ := syscall.UTF16PtrFromString(remote)
	userPtr, _ := syscall.UTF16PtrFromString(username)
	passPtr, _ := syscall.UTF16PtrFromString(password)

	nr := netResource{DwType: RESOURCETYPE_DISK, LpRemoteName: remotePtr}
	r1, _, e1 := procWNetAddConnection2W.Call(
		uintptr(unsafe.Pointer(&nr)),
		uintptr(unsafe.Pointer(passPtr)),
		uintptr(unsafe.Pointer(userPtr)),
		uintptr(CONNECT_TEMPORARY),
	)
	ret := uint32(r1)
	if ret != NO_ERROR {
		if e1 != syscall.Errno(0) {
			return ret, e1
		}
		return ret, syscall.Errno(ret)
	}
	return ret, nil
}

func isWinAccessError(err error) bool {
	if err == nil {
		return false
	}
	var perr *os.PathError
	if errors.As(err, &perr) {
		if errno, ok := perr.Err.(syscall.Errno); ok {
			return errno == ERROR_ACCESS_DENIED || errno == ERROR_LOGON_FAILURE
		}
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno == ERROR_ACCESS_DENIED || errno == ERROR_LOGON_FAILURE
	}
	return false
}

// ensureWindowsConnection attempts to establish a temporary connection for UNC path
// using keyring or prompting via CredentialsProvider. It never disconnects existing
// sessions. On credential conflict (1219) it returns the error without side effects.
func ensureWindowsConnection(p Parsed, native string) error {
	host := p.Host
	share := p.Share
	if host == "" || share == "" {
		up := parseUNC(native)
		host, share = up.Host, up.Share
	}
	if host == "" || share == "" {
		return errors.New("invalid UNC path")
	}
	// Get credentials: keyring first, then provider (UI)
	creds := getCredentials(host, share, "")
	if creds.Username == "" && creds.Password == "" && creds.Domain == "" {
		// No creds available
		return errors.New("no credentials provided")
	}
	user := creds.Username
	if creds.Domain != "" {
		user = creds.Domain + "\\" + user
	}
	ret, err := addConnection(host, share, user, creds.Password)
	if ret == NO_ERROR && err == nil {
		if creds.Persist && secretStore != nil {
			_ = secretStore.Set(host, share, creds.Domain, creds.Username, creds.Password)
		}
		return nil
	}
	// 1219 conflict: do not attempt any disconnects here; leave to caller/UI.
	return err
}

// IsWindowsCredentialConflict reports whether the error is a credential conflict (ERROR_SESSION_CREDENTIAL_CONFLICT=1219).
func IsWindowsCredentialConflict(err error) bool {
	if err == nil {
		return false
	}
	if errno, ok := err.(syscall.Errno); ok {
		return uint32(errno) == ERROR_SESSION_CREDENTIAL_CONFLICT
	}
	var perr *os.PathError
	if errors.As(err, &perr) {
		if errno, ok := perr.Err.(syscall.Errno); ok {
			return uint32(errno) == ERROR_SESSION_CREDENTIAL_CONFLICT
		}
	}
	return false
}
