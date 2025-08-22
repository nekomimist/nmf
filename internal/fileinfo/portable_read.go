package fileinfo

import (
	"os"
	"runtime"
)

// ReadDirPortable resolves the input path to a suitable provider and performs ReadDir.
func ReadDirPortable(p string) ([]os.DirEntry, error) {
	vfs, parsed, err := ResolveRead(p)
	if err != nil {
		return nil, err
	}
	native := parsed.Native
	if native == "" {
		native = p
	}
	entries, rerr := vfs.ReadDir(native)
	// Windows: if UNC access denied, try establishing a connection via keyring/UI then retry
	if rerr != nil && runtime.GOOS == "windows" && (isUNC(native) || parsed.Scheme == SchemeSMB) && isWinAccessError(rerr) {
		if err2 := ensureWindowsConnection(parsed, native); err2 == nil {
			return vfs.ReadDir(native)
		} else if IsWindowsCredentialConflict(err2) {
			// propagate credential conflict so caller can present guidance
			return nil, err2
		}
	}
	return entries, rerr
}
